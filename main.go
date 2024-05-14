package main

import (
	"context"
	"io/ioutil"
	"sync"

	"github.com/callebtc/electronwall/config"
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

type App struct {
	lnd    lndclient
	myInfo *lnrpc.GetInfoResponse
}

func NewApp(ctx context.Context, lnd lndclient) *App {
	myInfo, err := lnd.getMyInfo(ctx)
	if err != nil {
		log.Errorf("Could not get my node info: %s", err)
	}
	return &App{
		lnd:    lnd,
		myInfo: myInfo,
	}
}

// gets the lnd grpc connection
func getClientConnection(ctx context.Context) (*grpc.ClientConn, error) {
	creds, err := credentials.NewClientTLSFromFile(config.Configuration.TLSPath, "")
	if err != nil {
		return nil, err
	}
	macBytes, err := ioutil.ReadFile(config.Configuration.MacaroonPath)
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
	log.Infof("Connecting to %s", config.Configuration.Host)
	conn, err := grpc.DialContext(ctx, config.Configuration.Host, opts...)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func newLndClient(ctx context.Context) (*LndClient, error) {
	conn, err := getClientConnection(ctx)
	if err != nil {
		log.Errorf("Connection failed: %s", err)
		return &LndClient{}, err
	}
	client := lnrpc.NewLightningClient(conn)
	router := routerrpc.NewRouterClient(conn)
	return &LndClient{
		client: client,
		conn:   conn,
		router: router,
	}, nil
}

func main() {
	SetLogger(config.Configuration.Debug, config.Configuration.LogJson)
	Welcome()
	ctx := context.Background()
	for {
		lnd, err := newLndClient(ctx)
		if err != nil {
			log.Errorf("Failed to create lnd client: %s", err)
			return
		}

		app := NewApp(ctx, lnd)

		if len(app.myInfo.Alias) > 0 {
			log.Infof("Connected to %s (%s)", app.myInfo.Alias, trimPubKey([]byte(app.myInfo.IdentityPubkey)))
		} else {
			log.Infof("Connected to %s", app.myInfo.IdentityPubkey)
		}

		var wg sync.WaitGroup
		ctx = context.WithValue(ctx, ctxKeyWaitGroup, &wg)
		wg.Add(2)

		// channel acceptor
		if config.Configuration.ChannelMode != "passthrough" {
			app.DispatchChannelAcceptor(ctx)
		}

		// htlc acceptor
		if config.Configuration.ForwardMode != "passthrough" {
			app.DispatchHTLCAcceptor(ctx)
		}

		wg.Wait()
		log.Info("All routines stopped. Waiting for new connection.")
	}

}
