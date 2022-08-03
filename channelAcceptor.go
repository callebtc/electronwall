package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/lightningnetwork/lnd/lnrpc"
	log "github.com/sirupsen/logrus"
)

// DispatchChannelAcceptor is the channel acceptor event loop
func (app *App) DispatchChannelAcceptor(ctx context.Context) {
	// the channel event logger
	go func() {
		err := app.logChannelEvents(ctx)
		if err != nil {
			log.Error("channel event logger error",
				"err", err)
		}
	}()

	// the channel event interceptor
	go func() {
		err := app.interceptChannelEvents(ctx)
		if err != nil {
			log.Error("channel interceptor error",
				"err", err)
		}
		// release wait group for channel acceptor
		ctx.Value(ctxKeyWaitGroup).(*sync.WaitGroup).Done()
	}()

	log.Infof("[channel] Listening for incoming channel requests")

}

func (app *App) interceptChannelEvents(ctx context.Context) error {
	// get the lnd grpc connection
	acceptClient, err := app.lnd.channelAcceptor(ctx)
	if err != nil {
		panic(err)
	}
	for {
		req := lnrpc.ChannelAcceptRequest{}
		err = acceptClient.RecvMsg(&req)
		if err != nil {
			return err
		}

		// print the incoming channel request
		alias, err := app.lnd.getNodeAlias(ctx, hex.EncodeToString(req.NodePubkey))
		if err != nil {
			log.Errorf(err.Error())
		}
		var node_info_string string
		if alias != "" {
			node_info_string = fmt.Sprintf("%s (%s)", alias, hex.EncodeToString(req.NodePubkey))
		} else {
			node_info_string = hex.EncodeToString(req.NodePubkey)
		}
		log.Debugf("[channel] New channel request from %s", node_info_string)

		info, err := app.lnd.getNodeInfo(ctx, hex.EncodeToString(req.NodePubkey))
		if err != nil {
			log.Errorf(err.Error())
		}

		// determine mode and list of channels to parse
		var accept bool
		var listToParse []string
		if Configuration.ChannelMode == "allowlist" {
			accept = false
			listToParse = Configuration.ChannelAllowlist
		} else if Configuration.ChannelMode == "denylist" {
			accept = true
			listToParse = Configuration.ChannelDenylist
		}

		// parse and make decision
		for _, pubkey := range listToParse {
			if hex.EncodeToString(req.NodePubkey) == pubkey || pubkey == "*" {
				accept = !accept
				break
			}
		}

		var channel_info_string string
		if alias != "" {
			channel_info_string = fmt.Sprintf("(%d sat) from %s (%s, %d sat capacity, %d channels)",
				req.FundingAmt,
				alias,
				trimPubKey(req.NodePubkey),
				info.TotalCapacity,
				info.NumChannels,
			)
		} else {
			channel_info_string = fmt.Sprintf("(%d sat) from %s (%d sat capacity, %d channels)",
				req.FundingAmt,
				trimPubKey(req.NodePubkey),
				info.TotalCapacity,
				info.NumChannels,
			)
		}

		contextLogger := log.WithFields(log.Fields{
			"event":           "channel_request",
			"amount":          req.FundingAmt,
			"alias":           alias,
			"pubkey":          hex.EncodeToString(req.NodePubkey),
			"pending_chan_id": hex.EncodeToString(req.PendingChanId),
			"total_capacity":  info.TotalCapacity,
			"num_channels":    info.NumChannels,
		})

		res := lnrpc.ChannelAcceptResponse{}
		if accept {
			contextLogger.Infof("[channel] ✅ Allow channel %s", channel_info_string)
			res = lnrpc.ChannelAcceptResponse{Accept: true,
				PendingChanId:   req.PendingChanId,
				CsvDelay:        req.CsvDelay,
				MaxHtlcCount:    req.MaxAcceptedHtlcs,
				ReserveSat:      req.ChannelReserve,
				InFlightMaxMsat: req.MaxValueInFlight,
				MinHtlcIn:       req.MinHtlc,
			}

		} else {
			contextLogger.Infof("[channel] ❌ Deny channel %s", channel_info_string)
			res = lnrpc.ChannelAcceptResponse{Accept: false,
				Error: Configuration.ChannelRejectMessage}
		}
		err = acceptClient.Send(&res)
		if err != nil {
			log.Errorf(err.Error())
		}
	}

}

func (app *App) logChannelEvents(ctx context.Context) error {
	stream, err := app.lnd.subscribeChannelEvents(ctx, &lnrpc.ChannelEventSubscription{})
	if err != nil {
		return err
	}
	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}
		switch event.Type {
		case lnrpc.ChannelEventUpdate_OPEN_CHANNEL:
			alias, err := app.lnd.getNodeAlias(ctx, event.GetOpenChannel().RemotePubkey)
			if err != nil {
				log.Errorf(err.Error())
				alias = trimPubKey([]byte(event.GetOpenChannel().RemotePubkey))
			}
			channel_info_string := fmt.Sprintf("(%d sat) from %s",
				event.GetOpenChannel().Capacity,
				alias,
			)
			contextLogger := log.WithFields(log.Fields{
				"event":    "channel_open",
				"capacity": event.GetOpenChannel().Capacity,
				"alias":    alias,
				"pubkey":   event.GetOpenChannel().RemotePubkey,
				"chan_id":  ParseChannelID(event.GetOpenChannel().ChanId),
			})
			contextLogger.Infof("[channel] Opened channel %s %s", ParseChannelID(event.GetOpenChannel().ChanId), channel_info_string)
		}
		log.Tracef("[channel] Event: %s", event.String())
	}
}
