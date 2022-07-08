package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/lightningnetwork/lnd/macaroons"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

type key int

const (
	ctxKeyWaitGroup key = iota
)

// gets the lnd grpc connection
func getClientConnection(ctx context.Context) (*grpc.ClientConn, error) {
	creds, err := credentials.NewClientTLSFromFile(Configuration.TLSPath, "")
	if err != nil {
		return nil, err
	}
	macBytes, err := ioutil.ReadFile(Configuration.MacaroonPath)
	if err != nil {
		return nil, err
	}
	mac := &macaroon.Macaroon{}
	if err := mac.UnmarshalBinary(macBytes); err != nil {
		return nil, err
	}
	cred, err := macaroons.NewMacaroonCredential(mac)
	if err != nil {
		return nil, err
	}
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(cred),
	}
	conn, err := grpc.DialContext(ctx, Configuration.Host, opts...)
	if err != nil {
		return nil, err
	}
	log.Infof("Connected to %s", Configuration.Host)
	return conn, nil

}

func trimPubKey(pubkey []byte) string {
	return fmt.Sprintf("%s...%s", hex.EncodeToString(pubkey)[:6], hex.EncodeToString(pubkey)[len(hex.EncodeToString(pubkey))-6:])
}

func main() {
	ctx := context.Background()
	conn, err := getClientConnection(ctx)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	ctx = context.WithValue(ctx, ctxKeyWaitGroup, &wg)
	wg.Add(1)

	go dispatchChannelAcceptor(ctx, conn)

	// htlc interceptor

	log.Info("HTLC Interceptor registered")
	errc := make(chan error)
	go func() {
		errc <- processHtlcEvents(ctx, conn)
	}()

	go func() {
		errc <- processInterceptor(ctx, conn)
	}()

	log.Println(<-errc)

}
