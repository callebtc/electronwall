package main

import (
	"context"
	"math/rand"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func dispatchHTLCAcceptor(ctx context.Context, conn *grpc.ClientConn, client lnrpc.LightningClient) {
	router := routerrpc.NewRouterClient(conn)

	// htlc event subscriber, reports on incoming htlc events
	stream, err := router.SubscribeHtlcEvents(ctx, &routerrpc.SubscribeHtlcEventsRequest{})
	if err != nil {
		return
	}

	go func() {
		err := logHtlcEvents(stream)
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
		err := interceptHtlcEvents(interceptor)
		if err != nil {
			log.Error("interceptor error",
				"err", err)
		}
	}()

	log.Info("Listening for incoming HTLCs")
}

func logHtlcEvents(stream routerrpc.Router_SubscribeHtlcEventsClient) error {
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
			log.Infof("HTLC SettleEvent (chan_id:%d, htlc_id:%d)", event.IncomingChannelId, event.IncomingHtlcId)

		case *routerrpc.HtlcEvent_ForwardFailEvent:
			log.Infof("HTLC ForwardFailEvent (chan_id:%d, htlc_id:%d)", event.IncomingChannelId, event.IncomingHtlcId)

		case *routerrpc.HtlcEvent_ForwardEvent:
			log.Infof("HTLC ForwardEvent (chan_id:%d, htlc_id:%d)", event.IncomingChannelId, event.IncomingHtlcId)

		case *routerrpc.HtlcEvent_LinkFailEvent:
			log.Infof("HTLC LinkFailEvent (chan_id:%d, htlc_id:%d)", event.IncomingChannelId, event.IncomingHtlcId)
		}

	}
}

func interceptHtlcEvents(interceptor routerrpc.Router_HtlcInterceptorClient) error {
	for {
		event, err := interceptor.Recv()
		if err != nil {
			return err
		}

		// decision for routing
		log.Infof("Received HTLC. Making random decision...")
		accept := true
		if rand.Intn(10) < 8 {
			accept = false
		}

		response := &routerrpc.ForwardHtlcInterceptResponse{
			IncomingCircuitKey: event.IncomingCircuitKey,
		}

		if accept {
			log.Infof("✅ Accept HTLC (%d sat, htlc_id:%d, chan_id:%d->%d)", event.IncomingAmountMsat/1000, event.IncomingCircuitKey.HtlcId, event.IncomingCircuitKey.ChanId, event.OutgoingRequestedChanId)
			response.Action = routerrpc.ResolveHoldForwardAction_RESUME
		} else {
			log.Infof("❌ Reject HTLC (%d sat, htlc_id:%d, chan_id:%d->%d)", event.IncomingAmountMsat/1000, event.IncomingCircuitKey.HtlcId, event.IncomingCircuitKey.ChanId, event.OutgoingRequestedChanId)
			response.Action = routerrpc.ResolveHoldForwardAction_FAIL
		}

		err = interceptor.Send(response)
		if err != nil {
			return err
		}
	}
}
