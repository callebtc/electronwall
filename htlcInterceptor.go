package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func dispatchHTLCAcceptor(ctx context.Context) {
	conn := ctx.Value(connKey).(*grpc.ClientConn)
	router := routerrpc.NewRouterClient(conn)

	// htlc event subscriber, reports on incoming htlc events
	stream, err := router.SubscribeHtlcEvents(ctx, &routerrpc.SubscribeHtlcEventsRequest{})
	if err != nil {
		return
	}

	go func() {
		err := logHtlcEvents(ctx, stream)
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
		err := interceptHtlcEvents(ctx, interceptor)
		if err != nil {
			log.Error("interceptor error",
				"err", err)
		}
	}()

	log.Info("Listening for incoming HTLCs")
}

func logHtlcEvents(ctx context.Context, stream routerrpc.Router_SubscribeHtlcEventsClient) error {
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
			log.Debugf("HTLC SettleEvent (chan_id:%d, htlc_id:%d)", event.IncomingChannelId, event.IncomingHtlcId)

		case *routerrpc.HtlcEvent_ForwardFailEvent:
			log.Debugf("HTLC ForwardFailEvent (chan_id:%d, htlc_id:%d)", event.IncomingChannelId, event.IncomingHtlcId)

		case *routerrpc.HtlcEvent_ForwardEvent:
			log.Debugf("HTLC ForwardEvent (chan_id:%d, htlc_id:%d)", event.IncomingChannelId, event.IncomingHtlcId)

		case *routerrpc.HtlcEvent_LinkFailEvent:
			log.Debugf("HTLC LinkFailEvent (chan_id:%d, htlc_id:%d)", event.IncomingChannelId, event.IncomingHtlcId)
		}

	}
}

func interceptHtlcEvents(ctx context.Context, interceptor routerrpc.Router_HtlcInterceptorClient) error {
	for {
		event, err := interceptor.Recv()
		if err != nil {
			return err
		}
		go func() {
			// decision for routing
			decision_chan := make(chan bool, 1)
			go htlcInterceptDecision(ctx, event, decision_chan)

			channelEdge, err := getPubKeyFromChannel(ctx, event.IncomingCircuitKey.ChanId)
			if err != nil {
				log.Error("Error getting pubkey for channel %d", event.IncomingCircuitKey.ChanId)
			}
			alias, err := getNodeAlias(ctx, channelEdge.node1Pub.String())
			if err != nil {
				log.Errorf(err.Error())
			}

			var forward_info_string string
			if alias != "" {
				forward_info_string = fmt.Sprintf("from %s (%d sat, htlc_id:%d, chan_id:%d->%d)", alias, event.IncomingAmountMsat/1000, event.IncomingCircuitKey.HtlcId, event.IncomingCircuitKey.ChanId, event.OutgoingRequestedChanId)
			} else {
				forward_info_string = fmt.Sprintf("(%d sat, htlc_id:%d, chan_id:%d->%d)", event.IncomingAmountMsat/1000, event.IncomingCircuitKey.HtlcId, event.IncomingCircuitKey.ChanId, event.OutgoingRequestedChanId)
			}
			response := &routerrpc.ForwardHtlcInterceptResponse{
				IncomingCircuitKey: event.IncomingCircuitKey,
			}
			if <-decision_chan {
				log.Infof("✅ [forward-mode %s] Accept HTLC %s", Configuration.ForwardMode, forward_info_string)
				response.Action = routerrpc.ResolveHoldForwardAction_RESUME
			} else {
				log.Infof("❌ [forward-mode %s] Reject HTLC %s", Configuration.ForwardMode, forward_info_string)
				response.Action = routerrpc.ResolveHoldForwardAction_FAIL
			}
			err = interceptor.Send(response)
			if err != nil {
				return
			}
		}()
	}
}

func htlcInterceptDecision(ctx context.Context, event *routerrpc.ForwardHtlcInterceptRequest, decision_chan chan bool) {
	var accept bool

	if Configuration.ForwardMode == "whitelist" {
		accept = false
		for _, channel_id := range Configuration.ForwardWhitelist {
			chan_id_int, err := strconv.ParseUint(channel_id, 10, 64)
			if err != nil {
				log.Error("Error parsing channel id %s", channel_id)
				break
			}
			if event.IncomingCircuitKey.ChanId == chan_id_int {
				accept = true
				break
			}
		}
	} else if Configuration.ForwardMode == "blacklist" {
		accept = true
		for _, channel_id := range Configuration.ForwardBlacklist {
			chan_id_int, err := strconv.ParseUint(channel_id, 10, 64)
			if err != nil {
				log.Error("Error parsing channel id %s", channel_id)
				break
			}
			if event.IncomingCircuitKey.ChanId == chan_id_int {
				accept = false
				break
			}
		}
	}
	decision_chan <- accept
}

// Heavily inspired by by Joost Jager's circuitbreaker
func getNodeAlias(ctx context.Context, pubkey string) (string, error) {
	client := ctx.Value(clientKey).(lnrpc.LightningClient)
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

type channelEdge struct {
	node1Pub, node2Pub route.Vertex
}

func getPubKeyFromChannel(ctx context.Context, chan_id uint64) (*channelEdge, error) {
	client := ctx.Value(clientKey).(lnrpc.LightningClient)
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
