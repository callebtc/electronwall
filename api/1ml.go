package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

type OneML_NodeInfoResponse struct {
	LastUpdate int    `json:"last_update"`
	PubKey     string `json:"pub_key"`
	Alias      string `json:"alias"`
	Addresses  []struct {
		Network string `json:"network"`
		Addr    string `json:"addr"`
	} `json:"addresses"`
	Color        string `json:"color"`
	Capacity     int    `json:"capacity"`
	Channelcount int    `json:"channelcount"`
	Noderank     struct {
		Capacity     int `json:"capacity"`
		Channelcount int `json:"channelcount"`
		Age          int `json:"age"`
		Growth       int `json:"growth"`
		Availability int `json:"availability"`
	} `json:"noderank"`
}

type OneMlClient struct {
}

func GetOneMlClient() OneMlClient {
	return OneMlClient{}
}

func (c *OneMlClient) GetNodeInfo(pubkey string) (OneML_NodeInfoResponse, error) {

	url := fmt.Sprintf("https://1ml.com/node/%s/json", pubkey)

	log.Infof("Getting info from 1ml.com for %s", pubkey)

	client := http.Client{
		Timeout: time.Second * 2, // Timeout after 2 seconds
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}

	res, getErr := client.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	r := OneML_NodeInfoResponse{}
	jsonErr := json.Unmarshal(body, &r)
	if jsonErr != nil {
		log.Errorf("[1ml] api error: %v", jsonErr)
	}

	return r, nil
}
