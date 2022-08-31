package types

import (
	"github.com/callebtc/electronwall/api"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
)

type HtlcForwardEvent struct {
	PubkeyFrom string
	AliasFrom  string
	PubkeyTo   string
	AliasTo    string
	Event      *routerrpc.ForwardHtlcInterceptRequest
}

type ChannelAcceptEvent struct {
	PubkeyFrom string
	AliasFrom  string
	Event      *lnrpc.ChannelAcceptRequest
	NodeInfo   *lnrpc.NodeInfo
	OneMl      api.OneML_NodeInfoResponse
	Amboss     api.Amboss_NodeInfoResponse
}
