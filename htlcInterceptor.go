package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
	log "github.com/sirupsen/logrus"
)

func (app *app) dispatchHTLCAcceptor(ctx context.Context) {
	// wait group for channel acceptor
	defer ctx.Value(ctxKeyWaitGroup).(*sync.WaitGroup).Done()
	conn := app.conn
	router := routerrpc.NewRouterClient(conn)

	// htlc event subscriber, reports on incoming htlc events
	stream, err := router.SubscribeHtlcEvents(ctx, &routerrpc.SubscribeHtlcEventsRequest{})
	if err != nil {
		return
	}

	go func() {
		err := app.logHtlcEvents(ctx, stream)
		if err != nil {
			log.Error("htlc events error",
				"err", err)
		}
	}()

	// interceptor, decide whether to accept or reject
	interceptor, err := router.HtlcInterceptor(ctx)
	if err != nil {
		return
	}

	go func() {
		err := app.interceptHtlcEvents(ctx, interceptor)
		if err != nil {
			log.Error("interceptor error",
				"err", err)
		}
	}()

	log.Info("Listening for incoming HTLCs")
}

func (app *app) logHtlcEvents(ctx context.Context, stream routerrpc.Router_SubscribeHtlcEventsClient) error {
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
			log.Debugf("HTLC SettleEvent (chan_id:%s, htlc_id:%d)", parse_channelID(event.IncomingChannelId), event.IncomingHtlcId)

		case *routerrpc.HtlcEvent_ForwardFailEvent:
			log.Debugf("HTLC ForwardFailEvent (chan_id:%s, htlc_id:%d)", parse_channelID(event.IncomingChannelId), event.IncomingHtlcId)

		case *routerrpc.HtlcEvent_ForwardEvent:
			log.Debugf("HTLC ForwardEvent (chan_id:%s, htlc_id:%d)", parse_channelID(event.IncomingChannelId), event.IncomingHtlcId)

		case *routerrpc.HtlcEvent_LinkFailEvent:
			log.Debugf("HTLC LinkFailEvent (chan_id:%s, htlc_id:%d)", parse_channelID(event.IncomingChannelId), event.IncomingHtlcId)
		}

	}
}

func (app *app) interceptHtlcEvents(ctx context.Context, interceptor routerrpc.Router_HtlcInterceptorClient) error {
	for {
		event, err := interceptor.Recv()
		if err != nil {
			return err
		}
		go func() {
			// decision for routing
			decision_chan := make(chan bool, 1)
			go app.htlcInterceptDecision(ctx, event, decision_chan)

			channelEdge, err := app.getPubKeyFromChannel(ctx, event.IncomingCircuitKey.ChanId)
			if err != nil {
				log.Error("Error getting pubkey for channel %s", parse_channelID(event.IncomingCircuitKey.ChanId))
			}

			var pubkeyFrom, aliasFrom, pubkeyTo, aliasTo string
			if channelEdge.node1Pub.String() != app.myPubkey {
				pubkeyFrom = channelEdge.node1Pub.String()
			} else {
				pubkeyFrom = channelEdge.node2Pub.String()
			}
			aliasFrom, err = app.getNodeAlias(ctx, pubkeyFrom)
			if err != nil {
				aliasFrom = trimPubKey([]byte(pubkeyFrom))
				log.Error("Error getting alias for node %s", aliasFrom)
			}
			channelEdgeTo, err := app.getPubKeyFromChannel(ctx, event.OutgoingRequestedChanId)
			if err != nil {
				log.Error("Error getting pubkey for channel %s", parse_channelID(event.OutgoingRequestedChanId))
			}
			if channelEdgeTo.node1Pub.String() != app.myPubkey {
				pubkeyTo = channelEdgeTo.node1Pub.String()
			} else {
				pubkeyTo = channelEdgeTo.node2Pub.String()
			}
			aliasTo, err = app.getNodeAlias(ctx, pubkeyTo)
			if err != nil {
				aliasTo = trimPubKey([]byte(pubkeyTo))
				log.Error("Error getting alias for node %s", aliasTo)
			}

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
				log.Infof("✅ [forward %s] Allow HTLC %s", Configuration.ForwardMode, forward_info_string)
				response.Action = routerrpc.ResolveHoldForwardAction_RESUME
			} else {
				log.Infof("❌ [forward %s] Deny HTLC %s", Configuration.ForwardMode, forward_info_string)
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
func (app *app) htlcInterceptDecision(ctx context.Context, event *routerrpc.ForwardHtlcInterceptRequest, decision_chan chan bool) {
	var accept bool
	switch Configuration.ForwardMode {
	case "allowlist":
		accept = false
		for _, forward_allowlist_entry := range Configuration.ForwardAllowlist {
			if len(strings.Split(forward_allowlist_entry, "->")) == 2 {
				// check if channel_id is actually from-to channel
				split := strings.Split(forward_allowlist_entry, "->")
				from_channel_id, to_channel_id := split[0], split[1]
				if parse_channelID(event.IncomingCircuitKey.ChanId) == from_channel_id &&
					parse_channelID(event.OutgoingRequestedChanId) == to_channel_id {
					accept = true
					break
				}
			} else {
				// single entry
				if parse_channelID(event.IncomingCircuitKey.ChanId) == forward_allowlist_entry {
					accept = true
					break
				}
			}
		}
	case "denylist":
		accept = true
		for _, forward_allowlist_entry := range Configuration.ForwardAllowlist {
			if len(strings.Split(forward_allowlist_entry, "->")) == 2 {
				// check if channel_id is actually from-to channel
				split := strings.Split(forward_allowlist_entry, "->")
				from_channel_id, to_channel_id := split[0], split[1]
				if parse_channelID(event.IncomingCircuitKey.ChanId) == from_channel_id &&
					parse_channelID(event.OutgoingRequestedChanId) == to_channel_id {
					accept = false
					break
				}
			} else {
				// single entry
				if parse_channelID(event.IncomingCircuitKey.ChanId) == forward_allowlist_entry {
					accept = false
					break
				}
			}
		}
	default:
		err := fmt.Errorf("unknown forward mode: %s", Configuration.ForwardMode)
		panic(err)
	}
	decision_chan <- accept
}

// Heavily inspired by by Joost Jager's circuitbreaker
func (app *app) getNodeAlias(ctx context.Context, pubkey string) (string, error) {
	client := app.client
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	info, err := client.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{
		PubKey: pubkey,
	})
	if err != nil {
		return "", err
	}

	if info.Node == nil {
		return "", errors.New("node info not available")
	}

	return info.Node.Alias, nil
}

func (app *app) getMyPubkey(ctx context.Context) (string, error) {
	client := app.client
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	info, err := client.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		return "", err
	}

	return info.IdentityPubkey, nil
}

type channelEdge struct {
	node1Pub, node2Pub route.Vertex
}

func (app *app) getPubKeyFromChannel(ctx context.Context, chan_id uint64) (*channelEdge, error) {
	client := app.client
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	info, err := client.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{
		ChanId: chan_id,
	})
	if err != nil {
		return nil, err
	}

	node1Pub, err := route.NewVertexFromStr(info.Node1Pub)
	if err != nil {
		return nil, err
	}

	node2Pub, err := route.NewVertexFromStr(info.Node2Pub)
	if err != nil {
		return nil, err
	}

	return &channelEdge{
		node1Pub: node1Pub,
		node2Pub: node2Pub,
	}, nil
}
