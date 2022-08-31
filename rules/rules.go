package rules

import (
	"fmt"
	"os"

	"github.com/callebtc/electronwall/config"
	"github.com/callebtc/electronwall/types"
	"github.com/dop251/goja"
	log "github.com/sirupsen/logrus"
)

func Apply(s interface{}, decision_chan chan bool) (accept bool, err error) {

	if !config.Configuration.ApiRules.Apply {
		return true, nil
	}

	vm := goja.New()

	var js_script []byte

	// load script according to event type
	switch s.(type) {
	case types.HtlcForwardEvent:
		vm.Set("HtlcForward", s)
		js_script, err = os.ReadFile("rules/HtlcForward.js")
		if err != nil {
			log.Fatal(err)
		}
	case types.ChannelAcceptEvent:
		vm.Set("ChannelAccept", s)
		js_script, err = os.ReadFile("rules/ChannelAccept.js")
		if err != nil {
			log.Fatal(err)
		}
	default:
		return false, fmt.Errorf("no rule found for event type")
	}

	// execute script
	v, err := vm.RunString(string(js_script))
	if err != nil {
		log.Errorf("JS error: %v", err)
		return
	}

	accept = v.Export().(bool)
	decision_chan <- accept
	log.Infof("[rules] decision: %t", accept)
	return accept, nil
}
