package main

import (
	"encoding/hex"
	"fmt"

	log "github.com/sirupsen/logrus"
)

func trimPubKey(pubkey []byte) string {
	return fmt.Sprintf("%s...%s", hex.EncodeToString(pubkey)[:6], hex.EncodeToString(pubkey)[len(hex.EncodeToString(pubkey))-6:])
}

func welcome() {
	log.Info("---- ⚡️ electronwall 0.3 ⚡️ ----")
}

// setLogger will initialize the log format
func setLogger(debug bool) {
	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
}
