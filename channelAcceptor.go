package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/callebtc/electronwall/api"
	"github.com/callebtc/electronwall/rules"
	"github.com/callebtc/electronwall/types"
	"github.com/lightningnetwork/lnd/lnrpc"
	log "github.com/sirupsen/logrus"
)

func (app *App) GetChannelAcceptEvent(ctx context.Context, req lnrpc.ChannelAcceptRequest) (types.ChannelAcceptEvent, error) {
	// print the incoming channel request
	alias, err := app.lnd.getNodeAlias(ctx, hex.EncodeToString(req.NodePubkey))
	if err != nil {
		log.Errorf(err.Error())
	}

	info, err := app.lnd.getNodeInfo(ctx, hex.EncodeToString(req.NodePubkey))
	if err != nil {
		log.Errorf(err.Error())
	}

	noeInfo, err := api.GetApiNodeinfo(string(req.NodePubkey))
	if err != nil {
		log.Errorf(err.Error())
	}

	return types.ChannelAcceptEvent{
		PubkeyFrom: hex.EncodeToString(req.NodePubkey),
		AliasFrom:  alias,
		NodeInfo:   info,
		Event:      &req,
		OneMl:      noeInfo.OneMl,
		Amboss:     noeInfo.Amboss,
	}, nil
}

// DispatchChannelAcceptor is the channel acceptor event loop
func (app *App) DispatchChannelAcceptor(ctx context.Context) {
	// the channel event logger
	go func() {
		err := app.logChannelEvents(ctx)
		if err != nil {
			log.Errorf("channel event logger error: %v", err)
		}
	}()

	// the channel event interceptor
	go func() {
		err := app.interceptChannelEvents(ctx)
		if err != nil {
			log.Errorf("channel interceptor error: %v", err)
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

		channelAcceptEvent, err := app.GetChannelAcceptEvent(ctx, req)
		if err != nil {
			return err
		}

		var node_info_string string
		if channelAcceptEvent.AliasFrom != "" {
			node_info_string = fmt.Sprintf("%s (%s)", channelAcceptEvent.AliasFrom, hex.EncodeToString(channelAcceptEvent.Event.NodePubkey))
		} else {
			node_info_string = hex.EncodeToString(channelAcceptEvent.Event.NodePubkey)
		}
		log.Debugf("[channel] New channel request from %s", node_info_string)

		var channel_info_string string
		if channelAcceptEvent.AliasFrom != "" {
			channel_info_string = fmt.Sprintf("(%d sat) from %s (%s, %d sat capacity, %d channels)",
				channelAcceptEvent.Event.FundingAmt,
				channelAcceptEvent.AliasFrom,
				trimPubKey(channelAcceptEvent.Event.NodePubkey),
				channelAcceptEvent.NodeInfo.TotalCapacity,
				channelAcceptEvent.NodeInfo.NumChannels,
			)
		} else {
			channel_info_string = fmt.Sprintf("(%d sat) from %s (%d sat capacity, %d channels)",
				channelAcceptEvent.Event.FundingAmt,
				trimPubKey(channelAcceptEvent.Event.NodePubkey),
				channelAcceptEvent.NodeInfo.TotalCapacity,
				channelAcceptEvent.NodeInfo.NumChannels,
			)
		}

		contextLogger := log.WithFields(log.Fields{
			"event":           "channel_request",
			"amount":          channelAcceptEvent.Event.FundingAmt,
			"alias":           channelAcceptEvent.AliasFrom,
			"pubkey":          hex.EncodeToString(channelAcceptEvent.Event.NodePubkey),
			"pending_chan_id": hex.EncodeToString(channelAcceptEvent.Event.PendingChanId),
			"total_capacity":  channelAcceptEvent.NodeInfo.TotalCapacity,
			"num_channels":    channelAcceptEvent.NodeInfo.NumChannels,
		})

		// make decision
		decision_chan := make(chan bool, 1)
		rules_decision, err := rules.Apply(channelAcceptEvent, decision_chan)
		if err != nil {
			return err
		}
		// parse list
		list_decision, err := app.channelAcceptDecision(req)
		if err != nil {
			return err
		}

		accept := true
		if !rules_decision || !list_decision {
			accept = false
		}

		res := lnrpc.ChannelAcceptResponse{}
		if accept {
			if Configuration.LogJson {
				contextLogger.Infof("allow")
			} else {
				log.Infof("[channel] ✅ Allow channel %s", channel_info_string)
			}
			res = lnrpc.ChannelAcceptResponse{Accept: true,
				PendingChanId:   req.PendingChanId,
				CsvDelay:        req.CsvDelay,
				MaxHtlcCount:    req.MaxAcceptedHtlcs,
				ReserveSat:      req.ChannelReserve,
				InFlightMaxMsat: req.MaxValueInFlight,
				MinHtlcIn:       req.MinHtlc,
			}

		} else {
			if Configuration.LogJson {
				contextLogger.Infof("deny")
			} else {
				log.Infof("[channel] ❌ Deny channel %s", channel_info_string)
			}
			res = lnrpc.ChannelAcceptResponse{Accept: false,
				Error: Configuration.ChannelRejectMessage}
		}
		err = acceptClient.Send(&res)
		if err != nil {
			log.Errorf(err.Error())
		}
	}

}

func (app *App) channelAcceptDecision(req lnrpc.ChannelAcceptRequest) (bool, error) {
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
	log.Infof("TRYING %s", string(req.NodePubkey))
	for _, pubkey := range listToParse {
		if string(req.NodePubkey) == pubkey || pubkey == "*" {
			accept = !accept
			break
		}
	}
	return accept, nil

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

			if Configuration.LogJson {
				contextLogger := log.WithFields(log.Fields{
					"event":    "channel",
					"capacity": event.GetOpenChannel().Capacity,
					"alias":    alias,
					"pubkey":   event.GetOpenChannel().RemotePubkey,
					"chan_id":  ParseChannelID(event.GetOpenChannel().ChanId),
				})
				contextLogger.Infof("open")
			} else {
				log.Infof("[channel] Opened channel %s %s", ParseChannelID(event.GetOpenChannel().ChanId), channel_info_string)
			}
		}
		log.Tracef("[channel] Event: %s", event.String())
	}
}
