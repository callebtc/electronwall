package main

import (
	"context"
	"io/ioutil"
	"sync"

	"github.com/lightningnetwork/lnd/lnrpc"
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

type ContextKey string

var connKey ContextKey = "connKey"
var clientKey ContextKey = "clientKey"

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

func main() {
	ctx := context.Background()
	conn, err := getClientConnection(ctx)
	if err != nil {
		panic(err)
	}
	client := lnrpc.NewLightningClient(conn)

	var wg sync.WaitGroup
	ctx = context.WithValue(ctx, ctxKeyWaitGroup, &wg)
	wg.Add(1)

	ctx = context.WithValue(ctx, clientKey, client)
	ctx = context.WithValue(ctx, connKey, conn)

	// channel acceptor
	go dispatchChannelAcceptor(ctx)

	// htlc acceptor
	go dispatchHTLCAcceptor(ctx)

	wg.Wait()
}
