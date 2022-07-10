package main

import (
	"context"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

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

// --------------- HTLC Forward tests ---------------

func TestHTLCDenylist(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	Configuration.ForwardMode = "denylist"
	Configuration.ForwardDenylist = []string{"700762x1327x1->690757x1005x1"}

	app.DispatchHTLCAcceptor(ctx)
	time.Sleep(1 * time.Second)

	// both keys match: should be denied

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

	// both keys no match: should be allowed
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

	// wildcard out, both match: should be denied

	Configuration.ForwardDenylist = []string{"700762x1327x1->*"}

	key = &routerrpc.CircuitKey{
		ChanId: 770495967390531585,
		HtlcId: 1337,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 759495353533530113,
	}

	resp = <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_FAIL, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}

	// wildcard out, first key doesn't match: should be allowed

	Configuration.ForwardDenylist = []string{"700762x1327x1->*"}

	key = &routerrpc.CircuitKey{
		ChanId: 759495353533530113,
		HtlcId: 1337,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 759495353533530113,
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

	app.DispatchHTLCAcceptor(ctx)
	time.Sleep(1 * time.Second)

	Configuration.ForwardMode = "allowlist"

	// both keys correct: should be allowed
	Configuration.ForwardAllowlist = []string{"700762x1327x1->690757x1005x1"}
	log.Tracef("[test] Mode: %s, Rules: %v", Configuration.ForwardMode, Configuration.ForwardAllowlist)

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

	// both keys wrong: should be denied
	Configuration.ForwardAllowlist = []string{"700762x1327x1->690757x1005x1"}
	log.Tracef("[test] Mode: %s, Rules: %v", Configuration.ForwardMode, Configuration.ForwardAllowlist)

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

	// wildcard: should be allowed

	Configuration.ForwardAllowlist = []string{"*"}
	log.Tracef("[test] Mode: %s, Rules: %v", Configuration.ForwardMode, Configuration.ForwardAllowlist)

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

	// wildcard in: should be allowed

	Configuration.ForwardAllowlist = []string{"*->690757x1005x1"}
	log.Tracef("[test] Mode: %s, Rules: %v", Configuration.ForwardMode, Configuration.ForwardAllowlist)

	key = &routerrpc.CircuitKey{
		ChanId: 123456789876543210,
		HtlcId: 1337,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 759495353533530113,
	}

	resp = <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}

	// wildcard out: should be allowed

	Configuration.ForwardAllowlist = []string{"700762x1327x1->*"}
	log.Tracef("[test] Mode: %s, Rules: %v", Configuration.ForwardMode, Configuration.ForwardAllowlist)

	key = &routerrpc.CircuitKey{
		ChanId: 770495967390531585,
		HtlcId: 1337,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 123456789876543210,
	}

	resp = <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}

	// wildcard out but wrong in key: should be denied

	Configuration.ForwardAllowlist = []string{"700762x1327x1->*"}
	log.Tracef("[test] Mode: %s, Rules: %v", Configuration.ForwardMode, Configuration.ForwardAllowlist)

	key = &routerrpc.CircuitKey{
		ChanId: 123456789876543210,
		HtlcId: 1337,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 123456789876543210,
	}

	resp = <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_FAIL, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}

	// wildcard in but wrong out key: should be denied

	Configuration.ForwardAllowlist = []string{"*->700762x1327x1"}
	log.Tracef("[test] Mode: %s, Rules: %v", Configuration.ForwardMode, Configuration.ForwardAllowlist)

	key = &routerrpc.CircuitKey{
		ChanId: 123456789876543210,
		HtlcId: 1337,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 123456789876543210,
	}

	resp = <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_FAIL, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}

	// wildcard both: should be allowed

	Configuration.ForwardAllowlist = []string{"*->*"}
	log.Tracef("[test] Mode: %s, Rules: %v", Configuration.ForwardMode, Configuration.ForwardAllowlist)

	key = &routerrpc.CircuitKey{
		ChanId: 123456789876543210,
		HtlcId: 1337,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 123456789876543210,
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

// --------------- Channel accept tests ---------------

func TestChannelAllowlist(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	Configuration.ChannelMode = "allowlist"
	Configuration.ChannelAllowlist = []string{"6d792d7075626b65792d69732d766572792d6c6f6e672d666f722d7472696d6d696e672d7075626b6579"}

	app.DispatchChannelAcceptor(ctx)
	time.Sleep(1 * time.Second)

	// correct key: should be allowed

	client.channelAcceptorRequests <- &lnrpc.ChannelAcceptRequest{
		NodePubkey:    []byte("my-pubkey-is-very-long-for-trimming-pubkey"),
		FundingAmt:    1337,
		PendingChanId: []byte("759495353533530113"),
	}

	resp := <-client.channelAcceptorResponses
	require.Equal(t, resp.Accept, true)

	// wrong key: should be denied

	client.channelAcceptorRequests <- &lnrpc.ChannelAcceptRequest{
		NodePubkey:    []byte("WRONG PUBKEY"),
		FundingAmt:    1337,
		PendingChanId: []byte("759495353533530113"),
	}

	resp = <-client.channelAcceptorResponses
	require.Equal(t, resp.Accept, false)

	// wildcard: should be allowed

	Configuration.ChannelAllowlist = []string{"*"}

	client.channelAcceptorRequests <- &lnrpc.ChannelAcceptRequest{
		NodePubkey:    []byte("WRONG PUBKEY"),
		FundingAmt:    1337,
		PendingChanId: []byte("759495353533530113"),
	}

	resp = <-client.channelAcceptorResponses
	require.Equal(t, resp.Accept, true)
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

	// wildcard: should be denied

	Configuration.ChannelDenylist = []string{"*"}

	client.channelAcceptorRequests <- &lnrpc.ChannelAcceptRequest{
		NodePubkey:    []byte("WRONG PUBKEY"),
		FundingAmt:    1337,
		PendingChanId: []byte("759495353533530113"),
	}

	resp = <-client.channelAcceptorResponses
	require.Equal(t, resp.Accept, false)
}
