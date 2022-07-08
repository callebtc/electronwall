package main

import (
	"context"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func processHtlcEvents(ctx context.Context, conn *grpc.ClientConn) error {
	stream, err := routerrpc.NewRouterClient(conn).SubscribeHtlcEvents(ctx, &routerrpc.SubscribeHtlcEventsRequest{})
	if err != nil {
		return err
	}
	for i := 0; i < Configuration.Worker; i++ {
		go func() {
			for {
				event, err := stream.Recv()
				if err != nil {
					log.Printf("[stream.Recv] %v", err)
					continue
				}

				switch event.Event.(type) {
				case *routerrpc.HtlcEvent_SettleEvent:
					log.Infof("Event: Settle %d %d", event.IncomingChannelId, event.IncomingHtlcId)

				case *routerrpc.HtlcEvent_ForwardFailEvent:
					log.Infof("Event: ForwardFail %d %d", event.IncomingChannelId, event.IncomingHtlcId)
				}
			}

		}()
	}
	return nil
}

func processInterceptor(ctx context.Context, conn *grpc.ClientConn) error {
	interceptor, err := routerrpc.NewRouterClient(conn).HtlcInterceptor(ctx)
	if err != nil {
		return err
	}
	for i := 0; i < Configuration.Worker; i++ {
		go func() {
			for {
				event, err := interceptor.Recv()
				if err != nil {
					log.Printf("[interceptor.Recv] %v", err)
					continue
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
					log.Printf("[interceptor.Send] %v", err)
				}
			}
		}()
	}
	return nil
}
