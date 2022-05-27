package main

import (
	"fmt"

	"github.com/jinzhu/configor"
)

var Configuration = struct {
	Host         string   `yaml:"host"`
	MacaroonPath string   `yaml:"macaroon_path"`
	TLSPath      string   `yaml:"tls_path"`
	Accept       []string `yaml:"accept"`
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
}
