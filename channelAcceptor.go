package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"sync"

	"github.com/lightningnetwork/lnd/lnrpc"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func dispatchChannelAcceptor(ctx context.Context, conn *grpc.ClientConn, client lnrpc.LightningClient) {
	// wait group for channel acceptor
	defer ctx.Value(ctxKeyWaitGroup).(*sync.WaitGroup).Done()
	// get the lnd grpc connection
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
			log.Infof("✅ [%s mode] Allow channel from %s (chan_id: %d)", Configuration.Mode, trimPubKey(req.NodePubkey), binary.BigEndian.Uint64(req.PendingChanId))
			res = lnrpc.ChannelAcceptResponse{Accept: true,
				PendingChanId:   req.PendingChanId,
				CsvDelay:        req.CsvDelay,
				MaxHtlcCount:    req.MaxAcceptedHtlcs,
				ReserveSat:      req.ChannelReserve,
				InFlightMaxMsat: req.MaxValueInFlight,
				MinHtlcIn:       req.MinHtlc,
			}

		} else {
			log.Infof("❌ [%s mode] Reject channel from %s", Configuration.Mode, trimPubKey(req.NodePubkey))
			res = lnrpc.ChannelAcceptResponse{Accept: false,
				Error: Configuration.RejectMessage}
		}
		err = acceptClient.Send(&res)
		if err != nil {
			log.Errorf(err.Error())
		}
	}
}
