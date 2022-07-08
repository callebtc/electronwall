package main

import (
	"context"
	"encoding/hex"
	"sync"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func dispatchHTLCInterceptor(ctx context.Context, conn *grpc.ClientConn) {
	// wait group for channel acceptor
	defer ctx.Value(ctxKeyWaitGroup).(*sync.WaitGroup).Done()
	// get the lnd grpc connection
	client := lnrpc.NewLightningClient(conn)

	acceptClient, err := client.ChannelAcceptor(ctx)
	if err != nil {
		panic(err)
	}
	log.Infof("Listening for incoming channel requests")
	for {
		req := lnrpc.ChannelAcceptRequest{}
		err = acceptClient.RecvMsg(&req)
		if err != nil {
			log.Errorf(err.Error())
		}
		log.Infof("New channel request from %s", hex.EncodeToString(req.NodePubkey))

		var accept bool

		if Configuration.Mode == "whitelist" {
			accept = false
			for _, pubkey := range Configuration.Whitelist {
				if hex.EncodeToString(req.NodePubkey) == pubkey {
					accept = true
					break
				}
			}
		} else if Configuration.Mode == "blacklist" {
			accept = true
			for _, pubkey := range Configuration.Blacklist {
				if hex.EncodeToString(req.NodePubkey) == pubkey {
					accept = false
					break
				}
			}
		}

		res := lnrpc.ChannelAcceptResponse{}
		if accept {
			log.Infof("✅ [%s mode] Allow channel from %s", Configuration.Mode, trimPubKey(req.NodePubkey))
			res = lnrpc.ChannelAcceptResponse{Accept: true,
				PendingChanId:   req.PendingChanId,
				CsvDelay:        req.CsvDelay,
				MaxHtlcCount:    req.MaxAcceptedHtlcs,
				ReserveSat:      req.ChannelReserve,
				InFlightMaxMsat: req.MaxValueInFlight,
				MinHtlcIn:       req.MinHtlc,
			}

		} else {
			log.Infof("❌ [%s mode] Deny channel from %s", Configuration.Mode, trimPubKey(req.NodePubkey))
			res = lnrpc.ChannelAcceptResponse{Accept: false,
				Error: Configuration.RejectMessage}
		}
		err = acceptClient.Send(&res)
		if err != nil {
			log.Errorf(err.Error())
		}
	}
}

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
			log.Infof("Settle %s %s", event.IncomingChannelId, event.IncomingHtlcId)

		case *routerrpc.HtlcEvent_ForwardFailEvent:
			log.Infof("ForwardFail %s %s", event.IncomingChannelId, event.IncomingHtlcId)
		}
	}
}

func processInterceptor(interceptor routerrpc.Router_HtlcInterceptorClient) error {
	for {
		event, err := interceptor.Recv()
		if err != nil {
			return err
		}

		// resumeChan := make(chan bool)

		print("I'm here")
		// p.interceptChan <- interceptEvent{
		// 	circuitKey: circuitKey{
		// 		channel: event.IncomingCircuitKey.ChanId,
		// 		htlc:    event.IncomingCircuitKey.HtlcId,
		// 	},
		// 	valueMsat: int64(event.OutgoingAmountMsat),
		// 	resume:    resumeChan,
		// }

		// resume, ok := <-resumeChan
		// if !ok {
		// 	return errors.New("resume channel closed")
		// }
		resume := true

		response := &routerrpc.ForwardHtlcInterceptResponse{
			IncomingCircuitKey: event.IncomingCircuitKey,
		}
		if resume {
			response.Action = routerrpc.ResolveHoldForwardAction_RESUME
		} else {
			response.Action = routerrpc.ResolveHoldForwardAction_FAIL
		}

		err = interceptor.Send(response)
		if err != nil {
			return err
		}
	}
}
