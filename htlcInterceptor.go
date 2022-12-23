package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/callebtc/electronwall/config"
	"github.com/callebtc/electronwall/rules"
	"github.com/callebtc/electronwall/types"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	log "github.com/sirupsen/logrus"
)

// getHtlcForwardEvent returns a struct containing relevant information about the current
// forward request that the decision engine can then use
func (app *App) getHtlcForwardEvent(ctx context.Context, event *routerrpc.ForwardHtlcInterceptRequest) (types.HtlcForwardEvent, error) {
	channelEdge, err := app.lnd.getPubKeyFromChannel(ctx, event.IncomingCircuitKey.ChanId)
	if err != nil {
		log.Errorf("[forward] Error getting pubkey for channel %s", ParseChannelID(event.IncomingCircuitKey.ChanId))
	}
	var pubkeyFrom, aliasFrom, pubkeyTo, aliasTo string
	if channelEdge.Node1Pub != app.myInfo.IdentityPubkey {
		pubkeyFrom = channelEdge.Node1Pub
	} else {
		pubkeyFrom = channelEdge.Node2Pub
	}
	aliasFrom, err = app.lnd.getNodeAlias(ctx, pubkeyFrom)
	if err != nil {
		aliasFrom = trimPubKey([]byte(pubkeyFrom))
		log.Errorf("[forward] Error getting alias for node %s", aliasFrom)
	}

	// we need to figure out which side of the channel is the other end
	channelEdgeTo, err := app.lnd.getPubKeyFromChannel(ctx, event.OutgoingRequestedChanId)
	if err != nil {
		log.Errorf("[forward] Error getting pubkey for channel %s", ParseChannelID(event.OutgoingRequestedChanId))
		return types.HtlcForwardEvent{}, err
	}
	if channelEdgeTo.Node1Pub != app.myInfo.IdentityPubkey {
		pubkeyTo = channelEdgeTo.Node1Pub
	} else {
		pubkeyTo = channelEdgeTo.Node2Pub
	}
	aliasTo, err = app.lnd.getNodeAlias(ctx, pubkeyTo)
	if err != nil {
		aliasTo = trimPubKey([]byte(pubkeyTo))
		log.Errorf("[forward] Error getting alias for node %s", aliasTo)
	}

	return types.HtlcForwardEvent{
		PubkeyFrom: pubkeyFrom,
		AliasFrom:  aliasFrom,
		PubkeyTo:   pubkeyTo,
		AliasTo:    aliasTo,
		Event:      event,
	}, nil
}

// DispatchHTLCAcceptor is the HTLC acceptor event loop
func (app *App) DispatchHTLCAcceptor(ctx context.Context) {
	go func() {
		err := app.logHtlcEvents(ctx)
		if err != nil {
			log.Error("htlc event logger error",
				"err", err)
		}
	}()

	go func() {
		err := app.interceptHtlcEvents(ctx)
		if err != nil {
			log.Error("htlc interceptor error",
				"err", err)
		}
		// release wait group for htlc interceptor
		ctx.Value(ctxKeyWaitGroup).(*sync.WaitGroup).Done()
	}()

	log.Info("[forward] Listening for incoming HTLCs")
}

// interceptHtlcEvents intercepts incoming htlc events
func (app *App) interceptHtlcEvents(ctx context.Context) error {
	// interceptor, decide whether to accept or reject
	interceptor, err := app.lnd.htlcInterceptor(ctx)
	if err != nil {
		return err
	}
	for {
		event, err := interceptor.Recv()
		if err != nil {
			return err
		}
		go func() {

			log.Tracef("[forward] HTLC event (%d->%d)", event.IncomingCircuitKey.ChanId, event.OutgoingRequestedChanId)
			htlcForwardEvent, err := app.getHtlcForwardEvent(ctx, event)
			if err != nil {
				return
			}

			forward_info_string := fmt.Sprintf(
				"from %s to %s (%d sat, chan_id:%s->%s, htlc_id:%d)",
				htlcForwardEvent.AliasFrom,
				htlcForwardEvent.AliasTo,
				event.IncomingAmountMsat/1000,
				ParseChannelID(event.IncomingCircuitKey.ChanId),
				ParseChannelID(event.OutgoingRequestedChanId),
				event.IncomingCircuitKey.HtlcId,
			)

			contextLogger := log.WithFields(log.Fields{
				"event":       "forward_request",
				"in_alias":    htlcForwardEvent.AliasFrom,
				"out_alias":   htlcForwardEvent.AliasTo,
				"amount":      event.IncomingAmountMsat / 1000,
				"in_chan_id":  ParseChannelID(event.IncomingCircuitKey.ChanId),
				"out_chan_id": ParseChannelID(event.OutgoingRequestedChanId),
				"htlc_id":     event.IncomingCircuitKey.HtlcId,
			})

			// decision for routing
			decision_chan := make(chan bool, 1)

			list_decision, err := app.htlcInterceptDecision(ctx, event, decision_chan)
			if err != nil {
				return
			}
			rules_decision, err := rules.Apply(htlcForwardEvent, decision_chan)
			if err != nil {
				fmt.Printf("script error: %v", err)
				return
			}

			accept := true
			if !list_decision || !rules_decision {
				accept = false
			}

			response := &routerrpc.ForwardHtlcInterceptResponse{
				IncomingCircuitKey: event.IncomingCircuitKey,
			}
			switch accept {
			case true:
				if config.Configuration.LogJson {
					contextLogger.Infof("allow")
				} else {
					log.Infof("[forward] ✅ Allow HTLC %s", forward_info_string)
				}
				response.Action = routerrpc.ResolveHoldForwardAction_RESUME
			case false:
				if config.Configuration.LogJson {
					contextLogger.Infof("deny")
				} else {
					log.Infof("[forward] ❌ Deny HTLC %s", forward_info_string)
				}
				response.Action = routerrpc.ResolveHoldForwardAction_FAIL
			}
			err = interceptor.Send(response)
			if err != nil {
				return
			}
		}()
	}
}

// htlcInterceptDecision implements the rules upon which the
// decision is made whether or not to relay an HTLC to the next
// peer.
// The decision is made based on the following rules:
// 1. Either use a allowlist or a denylist.
// 2. If a single channel ID is used (12320768x65536x0), check the incoming ID of the HTLC against the list.
// 3. If two channel IDs are used (7929856x65537x0->7143424x65537x0), check the incoming ID and the outgoing ID of the HTLC against the list.
func (app *App) htlcInterceptDecision(ctx context.Context, event *routerrpc.ForwardHtlcInterceptRequest, decision_chan chan bool) (bool, error) {
	var accept bool
	var listToParse []string

	// // sleep for 60 seconds
	// time.Sleep(60 * time.Second)

	// determine filtering mode and list to parse
	switch config.Configuration.ForwardMode {
	case "allowlist":
		accept = false
		listToParse = config.Configuration.ForwardAllowlist
	case "denylist":
		accept = true
		listToParse = config.Configuration.ForwardDenylist
	default:
		return false, fmt.Errorf("unknown forward mode: %s", config.Configuration.ForwardMode)
	}

	// parse list and decide
	for _, forward_list_entry := range listToParse {
		if forward_list_entry == "*" {
			accept = !accept
			break
		}
		if len(strings.Split(forward_list_entry, "->")) == 2 {
			// check if entry is a pair of from->to
			split := strings.Split(forward_list_entry, "->")
			from_channel_id, to_channel_id := split[0], split[1]
			if (ParseChannelID(event.IncomingCircuitKey.ChanId) == from_channel_id || from_channel_id == "*") &&
				(ParseChannelID(event.OutgoingRequestedChanId) == to_channel_id || to_channel_id == "*") {
				accept = !accept
				log.Tracef("[test] Incoming: %s <-> %s, Outgoing: %s <-> %s", ParseChannelID(event.IncomingCircuitKey.ChanId), from_channel_id, ParseChannelID(event.OutgoingRequestedChanId), to_channel_id)
				break
			}
		} else {
			// single entry
			if ParseChannelID(event.IncomingCircuitKey.ChanId) == forward_list_entry {
				accept = !accept
				break
			}
		}
	}
	// decision_chan <- accept
	log.Infof("[list] decision: %t", accept)
	return accept, nil
}

// logHtlcEvents reports on incoming htlc events
func (app *App) logHtlcEvents(ctx context.Context) error {
	// htlc event subscriber, reports on incoming htlc events
	stream, err := app.lnd.subscribeHtlcEvents(ctx, &routerrpc.SubscribeHtlcEventsRequest{})
	if err != nil {
		return err
	}
	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}

		// we only care about HTLC forward events
		if event.EventType != routerrpc.HtlcEvent_FORWARD {
			continue
		}

		contextLogger := func(event *routerrpc.HtlcEvent) *log.Entry {
			b, err := json.Marshal(event)
			if err != nil {
				panic(err)
			}
			return log.WithFields(log.Fields{
				"type":  "forward",
				"event": string(b),
			})
		}

		switch event.Event.(type) {
		case *routerrpc.HtlcEvent_SettleEvent:
			if config.Configuration.LogJson {
				contextLogger(event).Infof("SettleEvent")
				// contextLogger.Debugf("[forward] Preimage: %s", hex.EncodeToString(event.GetSettleEvent().Preimage))
			} else {
				log.Infof("[forward] ⚡️ HTLC SettleEvent (chan_id:%s, htlc_id:%d)", ParseChannelID(event.IncomingChannelId), event.IncomingHtlcId)
				if event.GetSettleEvent() != nil && event.GetSettleEvent().Preimage != nil {
					log.Debugf("[forward] Preimage: %s", hex.EncodeToString(event.GetSettleEvent().Preimage))
				}
			}

		case *routerrpc.HtlcEvent_ForwardFailEvent:
			if config.Configuration.LogJson {
				contextLogger(event).Infof("ForwardFailEvent")
				// contextLogger.Debugf("[forward] Reason: %s", event.GetForwardFailEvent())
			} else {
				log.Infof("[forward] HTLC ForwardFailEvent (chan_id:%s, htlc_id:%d)", ParseChannelID(event.IncomingChannelId), event.IncomingHtlcId)
				// log.Debugf("[forward] Reason: %s", event.GetForwardFailEvent().String())
			}

		case *routerrpc.HtlcEvent_ForwardEvent:
			if config.Configuration.LogJson {
				contextLogger(event).Infof("ForwardEvent")
			} else {
				log.Infof("[forward] HTLC ForwardEvent (chan_id:%s, htlc_id:%d)", ParseChannelID(event.IncomingChannelId), event.IncomingHtlcId)
			}
			// log.Infof("[forward] HTLC ForwardEvent (chan_id:%s, htlc_id:%d)", ParseChannelID(event.IncomingChannelId), event.IncomingHtlcId)
			// log.Debugf("[forward] Details: %s", event.GetForwardEvent().String())

		case *routerrpc.HtlcEvent_LinkFailEvent:
			if config.Configuration.LogJson {
				contextLogger(event).Infof("LinkFailEvent")
				// contextLogger(event).Debugf("[forward] Reason: %s", event.GetLinkFailEvent().FailureString)
			} else {
				log.Infof("[forward] HTLC LinkFailEvent (chan_id:%s, htlc_id:%d)", ParseChannelID(event.IncomingChannelId), event.IncomingHtlcId)
				log.Debugf("[forward] Reason: %s", event.GetLinkFailEvent().FailureString)
			}

		}

	}
}
