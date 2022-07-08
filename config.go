package main

import (
	"fmt"

	"github.com/jinzhu/configor"
	log "github.com/sirupsen/logrus"
)

var Configuration = struct {
	Mode          string   `yaml:"mode"`
	Host          string   `yaml:"host"`
	MacaroonPath  string   `yaml:"macaroon_path"`
	TLSPath       string   `yaml:"tls_path"`
	Whitelist     []string `yaml:"whitelist"`
	Blacklist     []string `yaml:"blacklist"`
	RejectMessage string   `yaml:"reject_message"`
	Workers       int      `yaml:"workers"`
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

	if len(Configuration.Whitelist) == 0 {
		panic(fmt.Errorf("no accepted pubkeys specified in config.yaml"))
	}

	if len(Configuration.RejectMessage) > 500 {
		log.Warnf("reject message is too long. Trimming to 500 characters.")
		Configuration.RejectMessage = Configuration.RejectMessage[:500]
	}
	if len(Configuration.Mode) == 0 {
		Configuration.Mode = "blacklist"
	}
	log.Infof("Running in %s mode", Configuration.Mode)
}
