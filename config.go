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
	ChannelWhitelist     []string `yaml:"channel-whitelist"`
	ChannelBlacklist     []string `yaml:"channel-blacklist"`
	ChannelRejectMessage string   `yaml:"channel-reject-message"`
	ForwardMode          string   `yaml:"forward-mode"`
	ForwardWhitelist     []string `yaml:"forward-whitelist"`
	ForwardBlacklist     []string `yaml:"forward-blacklist"`
}{}

func init() {
	err := configor.Load(&Configuration, "config.yaml")
	if err != nil {
		panic(err)
	}
	checkConfig()
}

func checkConfig() {
	setLogger(Configuration.Debug)
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
		Configuration.ChannelMode = "blacklist"
	}
	if Configuration.ChannelMode != "whitelist" && Configuration.ChannelMode != "blacklist" {
		panic(fmt.Errorf("channel mode must be either whitelist or blacklist"))
	}

	log.Infof("Channel acceptor running in %s mode", Configuration.ForwardMode)

	if len(Configuration.ForwardMode) == 0 {
		Configuration.ForwardMode = "blacklist"
	}
	if Configuration.ForwardMode != "whitelist" && Configuration.ForwardMode != "blacklist" {
		panic(fmt.Errorf("channel mode must be either whitelist or blacklist"))
	}

	log.Infof("HTLC forwarder running in %s mode", Configuration.ForwardMode)
}
