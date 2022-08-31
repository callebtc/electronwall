package main

import (
	"context"
	"github.com/callebtc/electronwall/config"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestApp(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	app.DispatchChannelAcceptor(ctx)
	app.DispatchHTLCAcceptor(ctx)

	cancel()
}

// --------------- HTLC Forward tests ---------------

// both keys match: should be denied
func TestHTLCDenylist_BothMatch(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	config.Configuration.ForwardMode = "denylist"
	config.Configuration.ForwardDenylist = []string{"700762x1327x1->690757x1005x1"}

	app.DispatchHTLCAcceptor(ctx)

	key := &routerrpc.CircuitKey{
		ChanId: 770495967390531585,
		HtlcId: 1337000,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 759495353533530113,
		OutgoingAmountMsat:      99999999,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_FAIL, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}
}

// both keys no match: should be allowed
func TestHTLCDenylist_BothNoMatch(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	config.Configuration.ForwardMode = "denylist"
	config.Configuration.ForwardDenylist = []string{"700762x1327x1->690757x1005x1"}

	app.DispatchHTLCAcceptor(ctx)

	key := &routerrpc.CircuitKey{
		ChanId: 123456789876543210,
		HtlcId: 1337000,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 9876543210123456543,
		OutgoingAmountMsat:      99999999,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}
}

// wildcard out, both match: should be denied
func TestHTLCDenylist_WildCardOutBothMatch(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	config.Configuration.ForwardMode = "denylist"
	config.Configuration.ForwardDenylist = []string{"700762x1327x1->690757x1005x1"}

	app.DispatchHTLCAcceptor(ctx)

	config.Configuration.ForwardDenylist = []string{"700762x1327x1->*"}

	key := &routerrpc.CircuitKey{
		ChanId: 770495967390531585,
		HtlcId: 1337000,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 759495353533530113,
		OutgoingAmountMsat:      99999999,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_FAIL, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}
}

// wildcard out, one doesn't match match: should be allowed
func TestHTLCDenylist_WildCardOutNoMatch(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	config.Configuration.ForwardMode = "denylist"
	config.Configuration.ForwardDenylist = []string{"700762x1327x1->690757x1005x1"}

	app.DispatchHTLCAcceptor(ctx)

	// wildcard out, first key doesn't match: should be allowed

	config.Configuration.ForwardDenylist = []string{"700762x1327x1->*"}

	key := &routerrpc.CircuitKey{
		ChanId: 759495353533530113,
		HtlcId: 1337000,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 759495353533530113,
		OutgoingAmountMsat:      99999999,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}
}

func TestHTLCAllowlist_BothMatch(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	app.DispatchHTLCAcceptor(ctx)

	config.Configuration.ForwardMode = "allowlist"

	// both keys correct: should be allowed
	config.Configuration.ForwardAllowlist = []string{"700762x1327x1->690757x1005x1"}
	log.Tracef("[test] Mode: %s, Rules: %v", config.Configuration.ForwardMode, config.Configuration.ForwardAllowlist)

	key := &routerrpc.CircuitKey{
		ChanId: 770495967390531585,
		HtlcId: 1337000,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 759495353533530113,
		OutgoingAmountMsat:      99999999,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}

}
func TestHTLCAllowlist_BothNonMatch(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	app.DispatchHTLCAcceptor(ctx)

	config.Configuration.ForwardMode = "allowlist"
	// both keys wrong: should be denied
	config.Configuration.ForwardAllowlist = []string{"700762x1327x1->690757x1005x1"}
	log.Tracef("[test] Mode: %s, Rules: %v", config.Configuration.ForwardMode, config.Configuration.ForwardAllowlist)

	key := &routerrpc.CircuitKey{
		ChanId: 123456789876543210,
		HtlcId: 1337000,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 9876543210123456543,
		OutgoingAmountMsat:      99999999,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_FAIL, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}
}
func TestHTLCAllowlist_Wildcard(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	app.DispatchHTLCAcceptor(ctx)

	config.Configuration.ForwardMode = "allowlist"
	// wildcard: should be allowed
	config.Configuration.ForwardAllowlist = []string{"*"}
	log.Tracef("[test] Mode: %s, Rules: %v", config.Configuration.ForwardMode, config.Configuration.ForwardAllowlist)
	key := &routerrpc.CircuitKey{
		ChanId: 123456789876543210,
		HtlcId: 1337000,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 9876543210123456543,
		OutgoingAmountMsat:      99999999,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}
}
func TestHTLCAllowlist_WildcardIn(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	app.DispatchHTLCAcceptor(ctx)

	config.Configuration.ForwardMode = "allowlist"
	// wildcard in: should be allowed

	config.Configuration.ForwardAllowlist = []string{"*->690757x1005x1"}
	log.Tracef("[test] Mode: %s, Rules: %v", config.Configuration.ForwardMode, config.Configuration.ForwardAllowlist)

	key := &routerrpc.CircuitKey{
		ChanId: 123456789876543210,
		HtlcId: 1337000,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 759495353533530113,
		OutgoingAmountMsat:      99999999,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}
}
func TestHTLCAllowlist_WildcardOut(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	app.DispatchHTLCAcceptor(ctx)

	config.Configuration.ForwardMode = "allowlist"
	// wildcard out: should be allowed
	config.Configuration.ForwardAllowlist = []string{"700762x1327x1->*"}
	log.Tracef("[test] Mode: %s, Rules: %v", config.Configuration.ForwardMode, config.Configuration.ForwardAllowlist)

	key := &routerrpc.CircuitKey{
		ChanId: 770495967390531585,
		HtlcId: 1337000,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 123456789876543210,
		OutgoingAmountMsat:      99999999,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}
}
func TestHTLCAllowlist_WildcardOutWrongKeyIn(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	app.DispatchHTLCAcceptor(ctx)

	config.Configuration.ForwardMode = "allowlist"
	// wildcard out but wrong in key: should be denied
	config.Configuration.ForwardAllowlist = []string{"700762x1327x1->*"}
	log.Tracef("[test] Mode: %s, Rules: %v", config.Configuration.ForwardMode, config.Configuration.ForwardAllowlist)

	key := &routerrpc.CircuitKey{
		ChanId: 123456789876543210,
		HtlcId: 1337000,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 123456789876543210,
		OutgoingAmountMsat:      99999999,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_FAIL, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}
}
func TestHTLCAllowlist_WildcardIn_WrongKeyOut(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	app.DispatchHTLCAcceptor(ctx)

	config.Configuration.ForwardMode = "allowlist"
	// wildcard in but wrong out key: should be denied
	config.Configuration.ForwardAllowlist = []string{"*->700762x1327x1"}
	log.Tracef("[test] Mode: %s, Rules: %v", config.Configuration.ForwardMode, config.Configuration.ForwardAllowlist)

	key := &routerrpc.CircuitKey{
		ChanId: 123456789876543210,
		HtlcId: 1337000,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 123456789876543210,
		OutgoingAmountMsat:      99999999,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_FAIL, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}
}
func TestHTLCAllowlist_WildcardBoth(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	app.DispatchHTLCAcceptor(ctx)

	config.Configuration.ForwardMode = "allowlist"
	// wildcard both: should be allowed
	config.Configuration.ForwardAllowlist = []string{"*->*"}
	log.Tracef("[test] Mode: %s, Rules: %v", config.Configuration.ForwardMode, config.Configuration.ForwardAllowlist)

	key := &routerrpc.CircuitKey{
		ChanId: 123456789876543210,
		HtlcId: 1337000,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey:      key,
		OutgoingRequestedChanId: 123456789876543210,
		OutgoingAmountMsat:      99999999,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	client.htlcEvents <- &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
		Event:             &routerrpc.HtlcEvent_SettleEvent{},
	}

}

// --------------- Channel accept tests ---------------

func TestChannelAllowlist_CorrectKey(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	config.Configuration.ChannelMode = "allowlist"
	config.Configuration.ChannelAllowlist = []string{"03006fcf3312dae8d068ea297f58e2bd00ec1ffe214b793eda46966b6294a53ce6"}

	app.DispatchChannelAcceptor(ctx)

	// correct key: should be allowed

	client.channelAcceptorRequests <- &lnrpc.ChannelAcceptRequest{
		NodePubkey:    []byte("03006fcf3312dae8d068ea297f58e2bd00ec1ffe214b793eda46966b6294a53ce6"),
		FundingAmt:    1337000,
		PendingChanId: []byte("759495353533530113"),
	}

	resp := <-client.channelAcceptorResponses
	require.Equal(t, resp.Accept, true)

}
func TestChannelAllowlist_WrongKey(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	config.Configuration.ChannelMode = "allowlist"
	config.Configuration.ChannelAllowlist = []string{"03006fcf3312dae8d068ea297f58e2bd00ec1ffe214b793eda46966b6294a53ce6"}

	app.DispatchChannelAcceptor(ctx)
	// wrong key: should be denied

	client.channelAcceptorRequests <- &lnrpc.ChannelAcceptRequest{
		NodePubkey:    []byte("WRONG-KEY"),
		FundingAmt:    1337000,
		PendingChanId: []byte("759495353533530113"),
	}

	resp := <-client.channelAcceptorResponses
	require.Equal(t, resp.Accept, false)
}
func TestChannelAllowlist_Wildcard(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	app.DispatchChannelAcceptor(ctx)

	// wildcard: should be allowed
	config.Configuration.ChannelMode = "allowlist"
	config.Configuration.ChannelAllowlist = []string{"*"}

	client.channelAcceptorRequests <- &lnrpc.ChannelAcceptRequest{
		NodePubkey:    []byte("03006fcf3312dae8d068ea297f58e2bd00ec1ffe214b793eda46966b6294a53ce6"),
		FundingAmt:    1337000,
		PendingChanId: []byte("759495353533530113"),
	}

	resp := <-client.channelAcceptorResponses
	require.Equal(t, resp.Accept, true)
}

func TestChannelDenylist_Match(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	config.Configuration.ChannelMode = "denylist"
	config.Configuration.ChannelDenylist = []string{"03006fcf3312dae8d068ea297f58e2bd00ec1ffe214b793eda46966b6294a53ce6"}

	app.DispatchChannelAcceptor(ctx)

	// should be denied

	client.channelAcceptorRequests <- &lnrpc.ChannelAcceptRequest{
		NodePubkey:    []byte("03006fcf3312dae8d068ea297f58e2bd00ec1ffe214b793eda46966b6294a53ce6"),
		FundingAmt:    1337000,
		PendingChanId: []byte("759495353533530113"),
	}

	resp := <-client.channelAcceptorResponses
	require.Equal(t, resp.Accept, false)
}

func TestChannelAllowlist_Match(t *testing.T) {
	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := NewApp(ctx, client)

	config.Configuration.ChannelMode = "allowlist"
	config.Configuration.ChannelAllowlist = []string{"03006fcf3312dae8d068ea297f58e2bd00ec1ffe214b793eda46966b6294a53ce6"}

	app.DispatchChannelAcceptor(ctx)

	// should be allowed
	client.channelAcceptorRequests <- &lnrpc.ChannelAcceptRequest{
		NodePubkey:    []byte("03006fcf3312dae8d068ea297f58e2bd00ec1ffe214b793eda46966b6294a53ce6"),
		FundingAmt:    1337000,
		PendingChanId: []byte("759495353533530113"),
	}

	resp := <-client.channelAcceptorResponses
	require.Equal(t, resp.Accept, true)
}
