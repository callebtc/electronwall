package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/stretchr/testify/require"
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

func TestApp(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	app.DispatchChannelAcceptor(ctx)
	app.DispatchHTLCAcceptor(ctx)

	time.Sleep(1 * time.Second)
	cancel()
}

func TestHTLCDenylist(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	Configuration.ForwardMode = "denylist"
	Configuration.ForwardDenylist = []string{"700762x1327x1->690757x1005x1"}

	app.DispatchHTLCAcceptor(ctx)
	time.Sleep(1 * time.Second)

	// should be denied

	key := &routerrpc.CircuitKey{
		ChanId: 770495967390531585,
		HtlcId: 1337,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 759495353533530113,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_FAIL, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}

	// should be allowed
	key = &routerrpc.CircuitKey{
		ChanId: 123456789876543210,
		HtlcId: 1337,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 9876543210123456543,
	}

	resp = <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}
}

func TestHTLCAllowlist(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	Configuration.ForwardMode = "allowlist"
	Configuration.ForwardAllowlist = []string{"700762x1327x1->690757x1005x1"}
	app.DispatchHTLCAcceptor(ctx)
	time.Sleep(1 * time.Second)

	// should be allowed

	key := &routerrpc.CircuitKey{
		ChanId: 770495967390531585,
		HtlcId: 1337,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 759495353533530113,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}

	// should be denied

	key = &routerrpc.CircuitKey{
		ChanId: 123456789876543210,
		HtlcId: 1337,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 9876543210123456543,
	}

	resp = <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_FAIL, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}
}

func TestChannelAllowlist(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	Configuration.ChannelMode = "allowlist"
	Configuration.ChannelAllowlist = []string{"6d792d7075626b65792d69732d766572792d6c6f6e672d666f722d7472696d6d696e672d7075626b6579"}

	app.DispatchChannelAcceptor(ctx)
	time.Sleep(1 * time.Second)

	// should be allowed

	client.channelAcceptorRequests <- &lnrpc.ChannelAcceptRequest{
		NodePubkey:    []byte("my-pubkey-is-very-long-for-trimming-pubkey"),
		FundingAmt:    1337,
		PendingChanId: []byte("759495353533530113"),
	}

	resp := <-client.channelAcceptorResponses
	require.Equal(t, resp.Accept, true)

	// should be denied

	client.channelAcceptorRequests <- &lnrpc.ChannelAcceptRequest{
		NodePubkey:    []byte("WRONG PUBKEY"),
		FundingAmt:    1337,
		PendingChanId: []byte("759495353533530113"),
	}

	resp = <-client.channelAcceptorResponses
	require.Equal(t, resp.Accept, false)
}

func TestChannelDenylist(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	Configuration.ChannelMode = "denylist"
	Configuration.ChannelDenylist = []string{"6d792d7075626b65792d69732d766572792d6c6f6e672d666f722d7472696d6d696e672d7075626b6579"}

	app.DispatchChannelAcceptor(ctx)
	time.Sleep(1 * time.Second)

	// should be denied

	client.channelAcceptorRequests <- &lnrpc.ChannelAcceptRequest{
		NodePubkey:    []byte("my-pubkey-is-very-long-for-trimming-pubkey"),
		FundingAmt:    1337,
		PendingChanId: []byte("759495353533530113"),
	}

	resp := <-client.channelAcceptorResponses
	require.Equal(t, resp.Accept, false)

	// should be allowed

	client.channelAcceptorRequests <- &lnrpc.ChannelAcceptRequest{
		NodePubkey:    []byte("WRONG PUBKEY"),
		FundingAmt:    1337,
		PendingChanId: []byte("759495353533530113"),
	}

	resp = <-client.channelAcceptorResponses
	require.Equal(t, resp.Accept, true)
}
