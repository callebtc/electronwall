package main

import (
	"context"
	"errors"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
	"google.golang.org/grpc"
)

type LndClient struct {
	client lnrpc.LightningClient
	conn   *grpc.ClientConn
	router routerrpc.RouterClient
}

type lndclient interface {
	getNodeInfo(ctx context.Context, pubkey string) (nodeInfo *lnrpc.NodeInfo, err error)
	getNodeAlias(ctx context.Context, pubkey string) (string, error)
	getMyInfo(ctx context.Context) (*lnrpc.GetInfoResponse, error)
	getPubKeyFromChannel(ctx context.Context, chan_id uint64) (*lnrpc.ChannelEdge, error)

	subscribeHtlcEvents(ctx context.Context,
		in *routerrpc.SubscribeHtlcEventsRequest) (
		routerrpc.Router_SubscribeHtlcEventsClient, error)
	htlcInterceptor(ctx context.Context) (
		routerrpc.Router_HtlcInterceptorClient, error)

	subscribeChannelEvents(ctx context.Context, in *lnrpc.ChannelEventSubscription) (
		lnrpc.Lightning_SubscribeChannelEventsClient, error)
	channelAcceptor(ctx context.Context) (
		lnrpc.Lightning_ChannelAcceptorClient, error)
}

// getNodeInfo returns the information of a node given a pubKey
func (lnd *LndClient) getNodeInfo(ctx context.Context, pubkey string) (
	nodeInfo *lnrpc.NodeInfo, err error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	info, err := lnd.client.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{
		PubKey: pubkey,
	})
	if err != nil {
		return &lnrpc.NodeInfo{}, err
	}
	return info, nil
}

// getNodeAlias returns the alias of a node pubkey
func (lnd *LndClient) getNodeAlias(ctx context.Context, pubkey string) (
	string, error) {
	info, err := lnd.getNodeInfo(ctx, pubkey)
	if err != nil {
		return "", err
	}

	if info.Node == nil {
		return "", errors.New("node info not available")
	}
	return info.Node.Alias, nil
}

// getMyPubkey returns the pubkey of my own node
func (lnd *LndClient) getMyInfo(ctx context.Context) (
	*lnrpc.GetInfoResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	info, err := lnd.client.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		return &lnrpc.GetInfoResponse{}, err
	}
	return info, nil
}

type channelEdge struct {
	node1Pub, node2Pub route.Vertex
}

// getPubKeyFromChannel returns the pubkey of the remote node in a channel
func (lnd *LndClient) getPubKeyFromChannel(ctx context.Context, chan_id uint64) (
	*lnrpc.ChannelEdge, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	info, err := lnd.client.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{
		ChanId: chan_id,
	})
	if err != nil {
		return nil, err
	}

	// node1Pub, err := route.NewVertexFromStr(info.Node1Pub)
	// if err != nil {
	// 	return nil, err
	// }

	// node2Pub, err := route.NewVertexFromStr(info.Node2Pub)
	// if err != nil {
	// 	return nil, err
	// }

	return &lnrpc.ChannelEdge{
		Node1Pub: info.Node1Pub,
		Node2Pub: info.Node2Pub,
	}, nil
}

func (lnd *LndClient) subscribeHtlcEvents(ctx context.Context,
	in *routerrpc.SubscribeHtlcEventsRequest) (
	routerrpc.Router_SubscribeHtlcEventsClient, error) {

	return lnd.router.SubscribeHtlcEvents(ctx, in)
}

func (lnd *LndClient) htlcInterceptor(ctx context.Context) (
	routerrpc.Router_HtlcInterceptorClient, error) {

	return lnd.router.HtlcInterceptor(ctx)
}

func (lnd *LndClient) subscribeChannelEvents(ctx context.Context, in *lnrpc.ChannelEventSubscription) (
	lnrpc.Lightning_SubscribeChannelEventsClient, error) {

	return lnd.client.SubscribeChannelEvents(ctx, in)

}

func (lnd *LndClient) channelAcceptor(ctx context.Context) (
	lnrpc.Lightning_ChannelAcceptorClient, error) {

	return lnd.client.ChannelAcceptor(ctx)

}

func (lnd *LndClient) close() {
	lnd.conn.Close()
}
