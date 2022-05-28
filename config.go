package main

import (
	"fmt"

	"github.com/jinzhu/configor"
	log "github.com/sirupsen/logrus"
)

var Configuration = struct {
	Host          string   `yaml:"host"`
	MacaroonPath  string   `yaml:"macaroon_path"`
	TLSPath       string   `yaml:"tls_path"`
	Accept        []string `yaml:"accept"`
	RejectMessage string   `yaml:"reject_message"`
}{}

func init() {
	err := configor.Load(&Configuration, "config.yaml")
	if err != nil {
		panic(err)
	}
	checkConfig()
}

func checkConfig() {
	if Configuration.Host == "" {
		panic(fmt.Errorf("no host specified in config.yaml"))
	}

	if len(Configuration.Accept) == 0 {
		panic(fmt.Errorf("no accepted pubkeys specified in config.yaml"))
	}

	if len(Configuration.RejectMessage) > 500 {
		log.Warnf("reject message is too long. Trimming to 500 characters.")
		Configuration.RejectMessage = Configuration.RejectMessage[:500]
	}
}
