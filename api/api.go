package api

import (
	"github.com/callebtc/electronwall/config"
	log "github.com/sirupsen/logrus"
)

type ApiClient interface {
	GetNodeInfo(pubkey string) OneML_NodeInfoResponse
}

type ApiNodeInfo struct {
	OneMl  OneML_NodeInfoResponse  `json:"1ml"`
	Amboss Amboss_NodeInfoResponse `json:"amboss"`
}

func GetApiNodeinfo(pubkey string) (ApiNodeInfo, error) {
	response := ApiNodeInfo{
		OneMl:  OneML_NodeInfoResponse{},
		Amboss: Amboss_NodeInfoResponse{},
	}

	if config.Configuration.ApiRules.OneMl.Active {
		// get info from 1ml
		OnemlClient := GetOneMlClient()
		onemlNodeInfo, err := OnemlClient.GetNodeInfo(pubkey)
		if err != nil {
			log.Errorf(err.Error())
			onemlNodeInfo = OneML_NodeInfoResponse{}
		}
		response.OneMl = onemlNodeInfo
	}

	if config.Configuration.ApiRules.Amboss.Active {
		// get info from amboss
		ambossClient := GetAmbossClient()
		ambossNodeInfo, err := ambossClient.GetNodeInfo(pubkey)
		if err != nil {
			log.Errorf(err.Error())
			ambossNodeInfo = Amboss_NodeInfoResponse{}
		}
		response.Amboss = ambossNodeInfo
	}
	return response, nil
}
