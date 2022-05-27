package main

import (
	"context"
	"encoding/hex"
	"io/ioutil"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/macaroons"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
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
func main() {
	conn, err := getClientConnection(context.Background())
	if err != nil {
		panic(err)
	}
	client := lnrpc.NewLightningClient(conn)
	acceptClient, err := client.ChannelAcceptor(context.Background())
	if err != nil {
		panic(err)
	}
	log.Infof("Listening for incoming channel requests...")
	for {
		req := lnrpc.ChannelAcceptRequest{}
		err = acceptClient.RecvMsg(&req)
		if err != nil {
			log.Errorf(err.Error())
		}
		log.Infof("New channel request from %s", hex.EncodeToString(req.NodePubkey))

		accept := false
		for _, pubkey := range Configuration.Accept {
			if hex.EncodeToString(req.NodePubkey) == pubkey {
				accept = true
				break
			}
		}

		res := lnrpc.ChannelAcceptResponse{}
		if accept {
			log.Infof("Accepting channel request from %s", hex.EncodeToString(req.NodePubkey))
			res = lnrpc.ChannelAcceptResponse{Accept: true,
				PendingChanId:   req.PendingChanId,
				CsvDelay:        req.CsvDelay,
				MaxHtlcCount:    req.MaxAcceptedHtlcs,
				ReserveSat:      req.ChannelReserve,
				InFlightMaxMsat: req.MaxValueInFlight,
				MinHtlcIn:       req.MinHtlc,
			}

		} else {
			log.Infof("Rejecting channel request from %s", hex.EncodeToString(req.NodePubkey))
			res = lnrpc.ChannelAcceptResponse{Accept: false}
		}
		err = acceptClient.Send(&res)
		if err != nil {
			log.Errorf(err.Error())
		}
	}

}
