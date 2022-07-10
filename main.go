package main

import (
	"context"
	"io/ioutil"
	"sync"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
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

type app struct {
	client   lnrpc.LightningClient
	conn     *grpc.ClientConn
	router   routerrpc.RouterClient
	myInfo   *lnrpc.GetInfoResponse
	myPubkey string
}

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
	log.Infof("Connecting to %s", Configuration.Host)
	conn, err := grpc.DialContext(ctx, Configuration.Host, opts...)
	if err != nil {
		return nil, err
	}

	return conn, nil

}

func main() {
	ctx := context.Background()
	for {
		conn, err := getClientConnection(ctx)
		if err != nil {
			log.Errorf("Could not connect to lnd: %s", err)
			return
		}
		client := lnrpc.NewLightningClient(conn)

		app := app{
			client: client,
			conn:   conn,
		}

		myInfo, err := app.getMyInfo(ctx)
		if err != nil {
			log.Errorf("Could not get my node info: %s", err)
			continue
		} else {
			app.myInfo = myInfo
			app.myPubkey = myInfo.IdentityPubkey

		}

		myAlias := app.myInfo.Alias
		if len(myAlias) > 0 {
			log.Infof("Connected to %s (%s)", myAlias, trimPubKey([]byte(app.myPubkey)))
		} else {
			log.Infof("Connected to %s", app.myPubkey)
		}

		var wg sync.WaitGroup
		ctx = context.WithValue(ctx, ctxKeyWaitGroup, &wg)
		wg.Add(2)

		// channel acceptor
		app.dispatchChannelAcceptor(ctx)

		// htlc acceptor
		app.dispatchHTLCAcceptor(ctx)

		wg.Wait()
		log.Info("All routines stopped. Waiting for new connection.")
	}

}
