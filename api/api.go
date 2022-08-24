package api

import log "github.com/sirupsen/logrus"

type ApiClient interface {
	GetNodeInfo(pubkey string) OneML_NodeInfoResponse
}

type ApiNodeInfo struct {
	OneMl  OneML_NodeInfoResponse
	Amboss Amboss_NodeInfoResponse
}

func GetApiNodeinfo(pubkey string) (ApiNodeInfo, error) {
	// get info from 1ml
	OnemlClient := GetOneMlClient()
	onemlNodeInfo, err := OnemlClient.GetNodeInfo(pubkey)
	if err != nil {
		log.Errorf(err.Error())
		onemlNodeInfo = OneML_NodeInfoResponse{}
	}

	// get info from amboss
	ambossClient := GetAmbossClient()
	ambossNodeInfo, err := ambossClient.GetNodeInfo(pubkey)
	if err != nil {
		log.Errorf(err.Error())
		ambossNodeInfo = Amboss_NodeInfoResponse{}
	}
	return ApiNodeInfo{
		OneMl:  onemlNodeInfo,
		Amboss: ambossNodeInfo,
	}, err
}
