package main

import (
	"fmt"

	"github.com/jinzhu/configor"
	log "github.com/sirupsen/logrus"
)

var Configuration = struct {
	ChannelMode          string   `yaml:"channel-mode"`
	Host                 string   `yaml:"host"`
	MacaroonPath         string   `yaml:"macaroon_path"`
	TLSPath              string   `yaml:"tls-path"`
	Debug                bool     `yaml:"debug"`
	LogJson              bool     `yaml:"log-json"`
	ChannelAllowlist     []string `yaml:"channel-allowlist"`
	ChannelDenylist      []string `yaml:"channel-denylist"`
	ChannelRejectMessage string   `yaml:"channel-reject-message"`
	ForwardMode          string   `yaml:"forward-mode"`
	ForwardAllowlist     []string `yaml:"forward-allowlist"`
	ForwardDenylist      []string `yaml:"forward-denylist"`
}{}

func init() {
	err := configor.Load(&Configuration, "config.yaml")
	if err != nil {
		panic(err)
	}
	checkConfig()
}

func checkConfig() {
	setLogger(Configuration.Debug, Configuration.LogJson)
	welcome()

	if Configuration.Host == "" {
		panic(fmt.Errorf("no host specified in config.yaml"))
	}
	if Configuration.MacaroonPath == "" {
		panic(fmt.Errorf("no macaroon path specified in config.yaml"))
	}
	if Configuration.TLSPath == "" {
		panic(fmt.Errorf("no tls path specified in config.yaml"))
	}

	if len(Configuration.ChannelRejectMessage) > 500 {
		log.Warnf("channel reject message is too long. Trimming to 500 characters.")
		Configuration.ChannelRejectMessage = Configuration.ChannelRejectMessage[:500]
	}

	if len(Configuration.ChannelMode) == 0 {
		Configuration.ChannelMode = "denylist"
	}
	if Configuration.ChannelMode != "allowlist" && Configuration.ChannelMode != "denylist" {
		panic(fmt.Errorf("channel mode must be either allowlist or denylist"))
	}

	log.Infof("Channel acceptor running in %s mode", Configuration.ChannelMode)

	if len(Configuration.ForwardMode) == 0 {
		Configuration.ForwardMode = "denylist"
	}
	if Configuration.ForwardMode != "allowlist" && Configuration.ForwardMode != "denylist" {
		panic(fmt.Errorf("channel mode must be either allowlist or denylist"))
	}

	log.Infof("HTLC forwarder running in %s mode", Configuration.ForwardMode)
}
