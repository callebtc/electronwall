# ⚡️🛡 electronwall
A tiny firewall for LND that can filter Lightning **channel opening requests** and **HTLC forwards** based on your custom rules. 

electronwall uses filter lists that either allow (allowlist) or reject (denylist) events from a list of node public keys for channel openings, or channel IDs and channel pairs for payment routings.

You can also write [custom rules](#programmable-rules) using a builtin Javascript engine. 

![electronwall0 4](https://user-images.githubusercontent.com/93376500/187682152-add9b2ee-7d84-4582-b5fd-3eb1a0fc7767.jpg)

## Install

### From source
Build from source (you may need to install go for this):

```bash
git clone https://github.com/callebtc/electronwall.git
cd electronwall
go build .
```

### Binaries

You can download a binary for your system [here](https://github.com/callebtc/electronwall/releases). You'll still need a [config file](https://github.com/callebtc/electronwall/blob/main/config.yaml.example).

## Config
Edit `config.yaml.example` and rename to `config.yaml`. 

## Run

```bash
./electronwall
```

# Rules

## Allowlist and denylist

Allowlist and denylist rules are set in `config.yaml` under the appropriate keys. See the [example](config.yaml.example) config. 

## Programmable rules

electronwall has a Javascript engine called [goja](https://github.com/dop251/goja) that allows you to set custom rules. Note that you can only use pure Javascript (ECMAScript), you can't import a ton of other dependcies like with web applications.

Rules are saved in the `rules/` directory. There are two files, one for channel open requests `ChannelAccept.js` and one for HTLC forwards `HtlcForward.js`.

electronwall passes [contextual information](#contextual-information) to the Javascript engine that you can use to create rich rules. See below for a list of objects that are currently supported.

 Here is one rather complex rule for channel accept decisions in `ChannelAccept.js` for demonstration purposes:

 ```javascript
 // only channels > 0.75 Msat
ChannelAccept.Event.FundingAmt >= 750000 && 
// nodes with high 1ML availability score
ChannelAccept.OneMl.Noderank.Availability > 100 &&
// nodes with a low enough 1ML age rank
ChannelAccept.OneMl.Noderank.Age < 10000 &&
( 
    // only nodes with Amboss contact data
    ChannelAccept.Amboss.Socials.Info.Email ||
    ChannelAccept.Amboss.Socials.Info.Twitter ||
    ChannelAccept.Amboss.Socials.Info.Telegram 
) &&
(
    // elitist: either nodes with amboss prime
    ChannelAccept.Amboss.Amboss.IsPrime ||
    // or nodes with high-ranking capacity
    ChannelAccept.Amboss.GraphInfo.Metrics.CapacityRank < 1000 ||
    // or nodes with high-ranking channel count
    ChannelAccept.Amboss.GraphInfo.Metrics.ChannelsRank < 1000
)
 ```

 Here is an example `HtlcForward.js`
 ```javascript
 if (
    // only forward amounts larger than 100 sat
    HtlcForward.Event.OutgoingAmountMsat >= 100000
) { true } else { false }
 ```

### Contextual information
Here is a list of all objects that are passed to the Javascript engine. You need to look at the structure of these objects in order to use them in a custom rule like the example above. 

#### LND: ChannelAcceptRequest `*.Event`
```go
type ChannelAcceptRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The pubkey of the node that wishes to open an inbound channel.
	NodePubkey []byte `protobuf:"bytes,1,opt,name=node_pubkey,json=nodePubkey,proto3" json:"node_pubkey,omitempty"`
	// The hash of the genesis block that the proposed channel resides in.
	ChainHash []byte `protobuf:"bytes,2,opt,name=chain_hash,json=chainHash,proto3" json:"chain_hash,omitempty"`
	// The pending channel id.
	PendingChanId []byte `protobuf:"bytes,3,opt,name=pending_chan_id,json=pendingChanId,proto3" json:"pending_chan_id,omitempty"`
	// The funding amount in satoshis that initiator wishes to use in the
	// channel.
	FundingAmt uint64 `protobuf:"varint,4,opt,name=funding_amt,json=fundingAmt,proto3" json:"funding_amt,omitempty"`
	// The push amount of the proposed channel in millisatoshis.
	PushAmt uint64 `protobuf:"varint,5,opt,name=push_amt,json=pushAmt,proto3" json:"push_amt,omitempty"`
	// The dust limit of the initiator's commitment tx.
	DustLimit uint64 `protobuf:"varint,6,opt,name=dust_limit,json=dustLimit,proto3" json:"dust_limit,omitempty"`
	// The maximum amount of coins in millisatoshis that can be pending in this
	// channel.
	MaxValueInFlight uint64 `protobuf:"varint,7,opt,name=max_value_in_flight,json=maxValueInFlight,proto3" json:"max_value_in_flight,omitempty"`
	// The minimum amount of satoshis the initiator requires us to have at all
	// times.
	ChannelReserve uint64 `protobuf:"varint,8,opt,name=channel_reserve,json=channelReserve,proto3" json:"channel_reserve,omitempty"`
	// The smallest HTLC in millisatoshis that the initiator will accept.
	MinHtlc uint64 `protobuf:"varint,9,opt,name=min_htlc,json=minHtlc,proto3" json:"min_htlc,omitempty"`
	// The initial fee rate that the initiator suggests for both commitment
	// transactions.
	FeePerKw uint64 `protobuf:"varint,10,opt,name=fee_per_kw,json=feePerKw,proto3" json:"fee_per_kw,omitempty"`
	//
	//The number of blocks to use for the relative time lock in the pay-to-self
	//output of both commitment transactions.
	CsvDelay uint32 `protobuf:"varint,11,opt,name=csv_delay,json=csvDelay,proto3" json:"csv_delay,omitempty"`
	// The total number of incoming HTLC's that the initiator will accept.
	MaxAcceptedHtlcs uint32 `protobuf:"varint,12,opt,name=max_accepted_htlcs,json=maxAcceptedHtlcs,proto3" json:"max_accepted_htlcs,omitempty"`
	// A bit-field which the initiator uses to specify proposed channel
	// behavior.
	ChannelFlags uint32 `protobuf:"varint,13,opt,name=channel_flags,json=channelFlags,proto3" json:"channel_flags,omitempty"`
	// The commitment type the initiator wishes to use for the proposed channel.
	CommitmentType CommitmentType `protobuf:"varint,14,opt,name=commitment_type,json=commitmentType,proto3,enum=lnrpc.CommitmentType" json:"commitment_type,omitempty"`
}

```

#### 1ML node information `*.OneMl`
```go
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
```
#### Amboss node information `*.Amboss`
```go
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
		IsPrime               bool `json:"is_prime"`
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
```

#### Network information `*.Network`
*TBD*
