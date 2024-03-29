package main

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"
)

func trimPubKey(pubkey []byte) string {
	N_SPLIT := 8
	if len(pubkey) > N_SPLIT {
		return fmt.Sprintf("%s..%s", hex.EncodeToString(pubkey)[:N_SPLIT/2], hex.EncodeToString(pubkey)[len(hex.EncodeToString(pubkey))-N_SPLIT/2:])
	} else {
		return hex.EncodeToString(pubkey)
	}
}

func Welcome() {
	log.Info("---- ⚡️ electronwall 0.4 ⚡️ ----")
}

// SetLogger will initialize the log format
func SetLogger(debug bool, json bool) {
	if json {
		customFormatter := new(log.JSONFormatter)
		customFormatter.TimestampFormat = "2006-01-02 15:04:05"
		customFormatter.PrettyPrint = true
		log.SetFormatter(customFormatter)
		return
	}

	log.SetOutput(os.Stdout)
	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	customFormatter.ForceColors = true

	log.SetFormatter(customFormatter)
}

func intTob64(i int64) string {
	return base64.RawURLEncoding.EncodeToString(big.NewInt(i).Bytes())
}

func intToHex(i int64) string {
	return hex.EncodeToString(big.NewInt(i).Bytes())
}

func ParseChannelID(e uint64) string {
	byte_e := big.NewInt(int64(e)).Bytes()
	hexstr := hex.EncodeToString(byte_e)
	if len(hexstr) < 12 {
		return ""
	}
	int_block3, _ := strconv.ParseInt(hexstr[:6], 16, 64)
	int_block2, _ := strconv.ParseInt(hexstr[6:12], 16, 64)
	int_block1, _ := strconv.ParseInt(hexstr[12:], 16, 64)
	return fmt.Sprintf("%dx%dx%d", int_block3, int_block2, int_block1)
}
