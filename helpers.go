package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/routing/route"
	log "github.com/sirupsen/logrus"
)

func trimPubKey(pubkey []byte) string {
	N_SPLIT := 8
	if len(pubkey) > N_SPLIT {
		return fmt.Sprintf("%s..%s", hex.EncodeToString(pubkey)[:N_SPLIT/2], hex.EncodeToString(pubkey)[len(hex.EncodeToString(pubkey))-N_SPLIT/2:])
	} else {
		return hex.EncodeToString(pubkey)
	}
}

func welcome() {
	log.Info("---- ⚡️ electronwall 0.3.1 ⚡️ ----")
}

// setLogger will initialize the log format
func setLogger(debug bool) {
	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
}

func intTob64(i int64) string {
	return base64.RawURLEncoding.EncodeToString(big.NewInt(i).Bytes())
}

func intToHex(i int64) string {
	return hex.EncodeToString(big.NewInt(i).Bytes())
}

func parse_channelID(e uint64) string {
	byte_e := big.NewInt(int64(e)).Bytes()
	hexstr := hex.EncodeToString(byte_e)
	int_block3, _ := strconv.ParseInt(hexstr[:6], 16, 64)
	int_block2, _ := strconv.ParseInt(hexstr[6:12], 16, 64)
	int_block1, _ := strconv.ParseInt(hexstr[12:], 16, 64)
	return fmt.Sprintf("%dx%dx%d", int_block3, int_block2, int_block1)
}

// Heavily inspired by by Joost Jager's circuitbreaker
func (app *app) getNodeInfo(ctx context.Context, pubkey string) (nodeInfo *lnrpc.NodeInfo, err error) {
	client := app.client
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	info, err := client.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{
		PubKey: pubkey,
	})
	if err != nil {
		return &lnrpc.NodeInfo{}, err
	}
	return info, nil
}

// getNodeAlias returns the alias of a node pubkey
func (app *app) getNodeAlias(ctx context.Context, pubkey string) (string, error) {
	info, err := app.getNodeInfo(ctx, pubkey)
	if err != nil {
		return "", err
	}

	if info.Node == nil {
		return "", errors.New("node info not available")
	}
	return info.Node.Alias, nil
}

// getMyPubkey returns the pubkey of my own node
func (app *app) getMyInfo(ctx context.Context) (*lnrpc.GetInfoResponse, error) {
	client := app.client
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	info, err := client.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		return &lnrpc.GetInfoResponse{}, err
	}
	return info, nil
}

type channelEdge struct {
	node1Pub, node2Pub route.Vertex
}

// getPubKeyFromChannel returns the pubkey of the remote node in a channel
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
