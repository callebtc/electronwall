package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	log "github.com/sirupsen/logrus"
)

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
			// decision for routing
			decision_chan := make(chan bool, 1)
			go app.htlcInterceptDecision(ctx, event, decision_chan)

			channelEdge, err := app.lnd.getPubKeyFromChannel(ctx, event.IncomingCircuitKey.ChanId)
			if err != nil {
				log.Errorf("[forward] Error getting pubkey for channel %s", parse_channelID(event.IncomingCircuitKey.ChanId))
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
				log.Errorf("[forward] Error getting pubkey for channel %s", parse_channelID(event.OutgoingRequestedChanId))
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

			log.Tracef("[forward] HTLC event (%d->%d)", event.IncomingCircuitKey.ChanId, event.OutgoingRequestedChanId)
			forward_info_string := fmt.Sprintf(
				"from %s to %s (%d sat, chan_id:%s->%s, htlc_id:%d)",
				aliasFrom,
				aliasTo,
				event.IncomingAmountMsat/1000,
				parse_channelID(event.IncomingCircuitKey.ChanId),
				parse_channelID(event.OutgoingRequestedChanId),
				event.IncomingCircuitKey.HtlcId,
			)

			response := &routerrpc.ForwardHtlcInterceptResponse{
				IncomingCircuitKey: event.IncomingCircuitKey,
			}
			if <-decision_chan {
				log.Infof("[forward] ✅ Allow HTLC %s", forward_info_string)
				response.Action = routerrpc.ResolveHoldForwardAction_RESUME
			} else {
				log.Infof("[forward] ❌ Deny HTLC %s", forward_info_string)
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
func (app *App) htlcInterceptDecision(ctx context.Context, event *routerrpc.ForwardHtlcInterceptRequest, decision_chan chan bool) {
	var accept bool
	var listToParse []string

	// determine filtering mode and list to parse
	switch Configuration.ForwardMode {
	case "allowlist":
		accept = false
		listToParse = Configuration.ForwardAllowlist
	case "denylist":
		accept = true
		listToParse = Configuration.ForwardDenylist
	default:
		err := fmt.Errorf("unknown forward mode: %s", Configuration.ForwardMode)
		panic(err)
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
			if (parse_channelID(event.IncomingCircuitKey.ChanId) == from_channel_id || from_channel_id == "*") &&
				(parse_channelID(event.OutgoingRequestedChanId) == to_channel_id || to_channel_id == "*") {
				accept = !accept
				log.Tracef("[test] Incoming: %s <-> %s, Outgoing: %s <-> %s", parse_channelID(event.IncomingCircuitKey.ChanId), from_channel_id, parse_channelID(event.OutgoingRequestedChanId), to_channel_id)
				break
			}
		} else {
			// single entry
			if parse_channelID(event.IncomingCircuitKey.ChanId) == forward_list_entry {
				accept = !accept
				break
			}
		}
	}
	decision_chan <- accept
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

		switch event.Event.(type) {
		case *routerrpc.HtlcEvent_SettleEvent:
			log.Debugf("[forward] ⚡️ HTLC SettleEvent (chan_id:%s, htlc_id:%d)", parse_channelID(event.IncomingChannelId), event.IncomingHtlcId)

		case *routerrpc.HtlcEvent_ForwardFailEvent:
			log.Debugf("[forward] HTLC ForwardFailEvent (chan_id:%s, htlc_id:%d)", parse_channelID(event.IncomingChannelId), event.IncomingHtlcId)

		case *routerrpc.HtlcEvent_ForwardEvent:
			log.Debugf("[forward] HTLC ForwardEvent (chan_id:%s, htlc_id:%d)", parse_channelID(event.IncomingChannelId), event.IncomingHtlcId)

		case *routerrpc.HtlcEvent_LinkFailEvent:
			log.Debugf("[forward] HTLC LinkFailEvent (chan_id:%s, htlc_id:%d)", parse_channelID(event.IncomingChannelId), event.IncomingHtlcId)
		}

	}
}
