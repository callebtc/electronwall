package api

import (
	"context"
	"time"

	"github.com/callebtc/electronwall/config"
	"github.com/machinebox/graphql"
	log "github.com/sirupsen/logrus"
)

type Amboss_NodeInfoResponse struct {
	Socials struct {
		Info struct {
			Email            string      `json:"email"`
			Telegram         string      `json:"telegram"`
			Twitter          string      `json:"twitter"`
			LightningAddress string      `json:"lightning_address"`
			Website          string      `json:"website"`
			Pubkey           string      `json:"pubkey"`
			MinChannelSize   interface{} `json:"minChannelSize"`
			Message          string      `json:"message"`
			TwitterVerified  bool        `json:"twitter_verified"`
			Updated          time.Time   `json:"updated"`
		} `json:"info"`
	} `json:"socials"`
	GraphInfo struct {
		LastUpdate time.Time `json:"last_update"`
		Metrics    struct {
			Capacity     string `json:"capacity"`
			CapacityRank int    `json:"capacity_rank"`
			Channels     int    `json:"channels"`
			ChannelsRank int    `json:"channels_rank"`
		} `json:"metrics"`
		Node struct {
			Addresses []struct {
				Addr   string `json:"addr"`
				IPInfo struct {
					City        string `json:"city"`
					Country     string `json:"country"`
					CountryCode string `json:"country_code"`
				} `json:"ip_info"`
				Network string `json:"network"`
			} `json:"addresses"`
			LastUpdate int    `json:"last_update"`
			Color      string `json:"color"`
			Features   []struct {
				FeatureID  string `json:"feature_id"`
				IsKnown    bool   `json:"is_known"`
				IsRequired bool   `json:"is_required"`
				Name       string `json:"name"`
			} `json:"features"`
		} `json:"node"`
	} `json:"graph_info"`
	Amboss struct {
		IsFavorite            bool `json:"is_favorite"`
		NumberFavorites       int  `json:"number_favorites"`
		NewChannelGossipDelta struct {
			Mean string `json:"mean"`
			Sd   string `json:"sd"`
		} `json:"new_channel_gossip_delta"`
		Notifications struct {
			NumberSubscribers int `json:"number_subscribers"`
		} `json:"notifications"`
	} `json:"amboss"`
}

type Amboss_NodeInfoResponse_Nested struct {
	Data struct {
		GetNode struct {
			Amboss_NodeInfoResponse
		} `json:"getNode"`
	} `json:"data"`
}

var amboss_graphql_query = `query Info($pubkey: String!) {
	getNode(pubkey: $pubkey) {
	  socials {
		info {
		  email
		  telegram
		  twitter
		  lightning_address
		  website
		  pubkey
		  minChannelSize
		  message
		  twitter_verified
		  updated
		}
	  }
	  graph_info {
		last_update
		metrics {
		  capacity
		  capacity_rank
		  channels
		  channels_rank
		}
		node {
		  addresses {
			addr
			ip_info {
			  city
			  country
			  country_code
			}
			network
		  }
		  last_update
		  color
		  features {
			feature_id
			is_known
			is_required
			name
		  }
		}
	  }
	  amboss {
		is_favorite
		number_favorites
		new_channel_gossip_delta {
		  mean
		  sd
		}
		notifications {
		  number_subscribers
		}
	  }
	}
  }`

var amboss_graphql_variabnes = `{
	"pubkey": "%s"
  }
`

type AmbossClient struct {
}

func GetAmbossClient() AmbossClient {
	return AmbossClient{}
}

func (c *AmbossClient) GetNodeInfo(pubkey string) (Amboss_NodeInfoResponse, error) {
	url := "https://api.amboss.space/graphql"
	log.Infof("Getting info from amboss.space for %s", pubkey)

	graphqlClient := graphql.NewClient(url)
	graphqlRequest := graphql.NewRequest(amboss_graphql_query)
	graphqlRequest.Var("pubkey", pubkey)
	// set header fields
	graphqlRequest.Header.Set("Cache-Control", "no-cache")
	graphqlRequest.Header.Set("Content-Type", "application/json")

	var r_nested Amboss_NodeInfoResponse_Nested
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(config.Configuration.ApiRules.Amboss.Timeout))
	defer cancel()

	if err := graphqlClient.Run(ctx, graphqlRequest, &r_nested.Data); err != nil {
		log.Errorf("[amboss] api error: %v", err)
	}

	r := r_nested.Data.GetNode.Amboss_NodeInfoResponse
	return r, nil
}
