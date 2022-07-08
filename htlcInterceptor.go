package main

import (
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	log "github.com/sirupsen/logrus"
)

func processHtlcEvents(stream routerrpc.Router_SubscribeHtlcEventsClient) error {
	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}

		if event.EventType != routerrpc.HtlcEvent_FORWARD {
			continue
		}

		switch event.Event.(type) {
		case *routerrpc.HtlcEvent_SettleEvent:
			log.Infof("Event: Settle %d %d", event.IncomingChannelId, event.IncomingHtlcId)

		case *routerrpc.HtlcEvent_ForwardFailEvent:
			log.Infof("Event: ForwardFail %d %d", event.IncomingChannelId, event.IncomingHtlcId)
		}
	}
}

func processInterceptor(interceptor routerrpc.Router_HtlcInterceptorClient) error {
	for {
		event, err := interceptor.Recv()
		if err != nil {
			return err
		}

		// decision for routing
		accept := false

		response := &routerrpc.ForwardHtlcInterceptResponse{
			IncomingCircuitKey: event.IncomingCircuitKey,
		}
		if accept {
			log.Infof("✅ Accept HTLC (%d sat, %s)", event.IncomingAmountMsat/1000, event.IncomingCircuitKey.String())
			response.Action = routerrpc.ResolveHoldForwardAction_RESUME
		} else {
			log.Infof("❌ Reject HTLC (%d sat, %s)", event.IncomingAmountMsat/1000, event.IncomingCircuitKey.String())
			response.Action = routerrpc.ResolveHoldForwardAction_FAIL
		}

		err = interceptor.Send(response)
		if err != nil {
			return err
		}
	}
}
