package rules

import (
	"fmt"
	"log"
	"os"

	"github.com/dop251/goja"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
)

func Apply(s interface{}, decision_chan chan bool) (accept bool, err error) {
	vm := goja.New()
	var js_script []byte

	// load script according to event type
	switch s.(type) {
	case lnrpc.ChannelAcceptRequest:
		vm.Set("ChannelAcceptRequest", s)
		js_script, err = os.ReadFile("rules/ChannelAcceptRequest.js")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(js_script))
	case *routerrpc.ForwardHtlcInterceptRequest:
		vm.Set("ForwardHtlcInterceptRequest", s)
		js_script, err = os.ReadFile("rules/ForwardHtlcInterceptRequest.js")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(js_script))
	default:
		return false, fmt.Errorf("no rule found for event type")
	}

	// execute script
	v, err := vm.RunString(string(js_script))
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	accept = v.Export().(bool)
	decision_chan <- accept
	return accept, nil
}
