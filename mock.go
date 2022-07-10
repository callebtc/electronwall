package main

import (
	"context"
	"errors"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
)

type lndclientMock struct {
	htlcEvents               chan *routerrpc.HtlcEvent
	htlcInterceptorRequests  chan *routerrpc.ForwardHtlcInterceptRequest
	htlcInterceptorResponses chan *routerrpc.ForwardHtlcInterceptResponse

	channelEvents            chan *lnrpc.ChannelEventUpdate
	channelAcceptorRequests  chan *lnrpc.ChannelAcceptRequest
	channelAcceptorResponses chan *lnrpc.ChannelAcceptResponse
}

func newLndclientMock() *lndclientMock {
	return &lndclientMock{
		htlcEvents:               make(chan *routerrpc.HtlcEvent),
		htlcInterceptorRequests:  make(chan *routerrpc.ForwardHtlcInterceptRequest),
		htlcInterceptorResponses: make(chan *routerrpc.ForwardHtlcInterceptResponse),
		channelAcceptorRequests:  make(chan *lnrpc.ChannelAcceptRequest),
		channelAcceptorResponses: make(chan *lnrpc.ChannelAcceptResponse),
	}
}

// --------------- Channel events mocks ---------------

type channelAcceptorMock struct {
	lnrpc.Lightning_ChannelAcceptorClient

	channelAcceptorRequests  chan *lnrpc.ChannelAcceptRequest
	channelAcceptorResponses chan *lnrpc.ChannelAcceptResponse
}

func (lnd *lndclientMock) channelAcceptor(ctx context.Context) (
	lnrpc.Lightning_ChannelAcceptorClient, error) {

	return &channelAcceptorMock{
		channelAcceptorRequests:  lnd.channelAcceptorRequests,
		channelAcceptorResponses: lnd.channelAcceptorResponses,
	}, nil

}

func (c *channelAcceptorMock) RecvMsg(m interface{}) error {
	req := <-c.channelAcceptorRequests
	*m.(*lnrpc.ChannelAcceptRequest) = *req
	return nil
}

func (c *channelAcceptorMock) Send(m *lnrpc.ChannelAcceptResponse) error {
	c.channelAcceptorResponses <- m
	return nil
}

type channelEventsMock struct {
	lnrpc.Lightning_SubscribeChannelEventsClient

	channelEvents chan *lnrpc.ChannelEventUpdate
}

func (h *channelEventsMock) Recv() (*lnrpc.ChannelEventUpdate, error) {
	event := <-h.channelEvents
	return event, nil
}

func (l *lndclientMock) subscribeChannelEvents(ctx context.Context,
	in *lnrpc.ChannelEventSubscription) (
	lnrpc.Lightning_SubscribeChannelEventsClient, error) {

	return &channelEventsMock{
		channelEvents: l.channelEvents,
	}, nil
}

// --------------- Node info mocks ---------------

// getNodeInfo returns the information of a node given a pubKey
func (lnd *lndclientMock) getNodeInfo(ctx context.Context, pubkey string) (
	nodeInfo *lnrpc.NodeInfo, err error) {
	info := &lnrpc.NodeInfo{
		Node: &lnrpc.LightningNode{
			Alias: "alias-" + trimPubKey([]byte(pubkey)),
		},
		NumChannels:   2,
		TotalCapacity: 1234,
	}
	return info, nil
}

// getNodeAlias returns the alias of a node pubkey
func (lnd *lndclientMock) getNodeAlias(ctx context.Context, pubkey string) (
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

func (lnd *lndclientMock) getMyInfo(ctx context.Context) (
	*lnrpc.GetInfoResponse, error) {
	info := &lnrpc.GetInfoResponse{
		IdentityPubkey: "my-pubkey-is-very-long-for-trimming-pubkey",
		Alias:          "my-alias",
	}
	return info, nil
}

func (lnd *lndclientMock) getPubKeyFromChannel(ctx context.Context, chan_id uint64) (
	*lnrpc.ChannelEdge, error) {
	return &lnrpc.ChannelEdge{
		Node1Pub: "my-pubkey-is-very-long-for-trimming-pubkey",
		Node2Pub: "other-pubkey-is-very-long-for-trimming-pubkey",
	}, nil
}

// --------------- HTLC events mock ---------------

type htlcEventsMock struct {
	routerrpc.Router_SubscribeHtlcEventsClient

	htlcEvents chan *routerrpc.HtlcEvent
}

func (h *htlcEventsMock) Recv() (*routerrpc.HtlcEvent, error) {
	event := <-h.htlcEvents
	return event, nil
}

type htlcInterceptorMock struct {
	routerrpc.Router_HtlcInterceptorClient

	htlcInterceptorRequests  chan *routerrpc.ForwardHtlcInterceptRequest
	htlcInterceptorResponses chan *routerrpc.ForwardHtlcInterceptResponse
}

func (h *htlcInterceptorMock) Send(resp *routerrpc.ForwardHtlcInterceptResponse) error {
	h.htlcInterceptorResponses <- resp
	return nil
}

func (h *htlcInterceptorMock) Recv() (*routerrpc.ForwardHtlcInterceptRequest, error) {
	event := <-h.htlcInterceptorRequests
	return event, nil
}

func (l *lndclientMock) subscribeHtlcEvents(ctx context.Context,
	in *routerrpc.SubscribeHtlcEventsRequest) (
	routerrpc.Router_SubscribeHtlcEventsClient, error) {

	return &htlcEventsMock{
		htlcEvents: l.htlcEvents,
	}, nil
}

func (l *lndclientMock) htlcInterceptor(ctx context.Context) (
	routerrpc.Router_HtlcInterceptorClient, error) {

	return &htlcInterceptorMock{
		htlcInterceptorRequests:  l.htlcInterceptorRequests,
		htlcInterceptorResponses: l.htlcInterceptorResponses,
	}, nil
}
