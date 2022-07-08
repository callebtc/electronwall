package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/lightningnetwork/lnd/lnrpc"
	log "github.com/sirupsen/logrus"
)

func dispatchChannelAcceptor(ctx context.Context) {
	client := ctx.Value(clientKey).(lnrpc.LightningClient)

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

		// print the incoming channel request
		alias, err := getNodeAlias(ctx, hex.EncodeToString(req.NodePubkey))
		if err != nil {
			log.Errorf(err.Error())
		}
		var node_info_string string
		if alias != "" {
			node_info_string = fmt.Sprintf("%s (%s)", alias, hex.EncodeToString(req.NodePubkey))
		} else {
			node_info_string = hex.EncodeToString(req.NodePubkey)
		}
		log.Debugf("New channel request from %s", node_info_string)

		var accept bool

		if Configuration.ChannelMode == "whitelist" {
			accept = false
			for _, pubkey := range Configuration.ChannelWhitelist {
				if hex.EncodeToString(req.NodePubkey) == pubkey {
					accept = true
					break
				}
			}
		} else if Configuration.ChannelMode == "blacklist" {
			accept = true
			for _, pubkey := range Configuration.ChannelBlacklist {
				if hex.EncodeToString(req.NodePubkey) == pubkey {
					accept = false
					break
				}
			}
		}

		var channel_info_string string
		if alias != "" {
			channel_info_string = fmt.Sprintf("from %s (%s, %d sat, chan_id:%d)", alias, trimPubKey(req.NodePubkey), req.FundingAmt, binary.BigEndian.Uint64(req.PendingChanId))
		} else {
			channel_info_string = fmt.Sprintf("from %s (%d sat, chan_id:%d)", trimPubKey(req.NodePubkey), req.FundingAmt, binary.BigEndian.Uint64(req.PendingChanId))
		}

		res := lnrpc.ChannelAcceptResponse{}
		if accept {
			log.Infof("✅ [channel-mode %s] Allow channel %s", Configuration.ChannelMode, channel_info_string)
			res = lnrpc.ChannelAcceptResponse{Accept: true,
				PendingChanId:   req.PendingChanId,
				CsvDelay:        req.CsvDelay,
				MaxHtlcCount:    req.MaxAcceptedHtlcs,
				ReserveSat:      req.ChannelReserve,
				InFlightMaxMsat: req.MaxValueInFlight,
				MinHtlcIn:       req.MinHtlc,
			}

		} else {
			log.Infof("❌ [channel-mode %s] Reject channel %s", Configuration.ChannelMode, channel_info_string)
			res = lnrpc.ChannelAcceptResponse{Accept: false,
				Error: Configuration.ChannelRejectMessage}
		}
		err = acceptClient.Send(&res)
		if err != nil {
			log.Errorf(err.Error())
		}
	}
}
