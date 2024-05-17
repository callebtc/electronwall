package main

import (
	"os"

	"github.com/callebtc/electronwall/api"
	"github.com/callebtc/electronwall/rules"
	"github.com/callebtc/electronwall/types"
	"github.com/lightningnetwork/lnd/lnrpc"
	log "github.com/sirupsen/logrus"
)

func main() {
	if len(os.Args) < 2 {
		log.Errorf("Pass node ID as argument.")
		return
	}
	pubkey := os.Args[1]
	log.Infof("pubkey: %s", pubkey)
	pk_byte := []byte(pubkey)
	event := types.ChannelAcceptEvent{}

	event.Event = &lnrpc.ChannelAcceptRequest{}

	event.Event.FundingAmt = 1_000_000
	log.Infof("Funding amount: %d sat", event.Event.FundingAmt)
	event.Event.NodePubkey = pk_byte

	req := lnrpc.ChannelAcceptRequest{}
	req.NodePubkey = pk_byte

	nodeInfo, err := api.GetApiNodeinfo(string(req.NodePubkey))
	if err != nil {
		log.Errorf(err.Error())
	}
	// log.Infoln("1ML")
	// // log.Infoln("%+v", noeInfo.OneMl)
	// log.Printf("%+v\n", nodeInfo.OneMl)
	// log.Infoln("Amboss")
	// log.Printf("%+v\n", nodeInfo.Amboss)

	event.OneMl = nodeInfo.OneMl
	event.Amboss = nodeInfo.Amboss

	rules_decision, err := rules.Apply(event)
	if err != nil {
		panic(err)
	}
	log.Infof("Decision: %t", rules_decision)
}
