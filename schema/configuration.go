package schema

import (
	"encoding/json"
	"errors"
	"time"
)

type APIConfig struct {
	Authenticated bool
	AllowedIPs    []string
	Username      string
	Password      string
	CORS          *string
	Enabled       bool
	HTTPHeaders   map[string]interface{}
	SSL           bool
	SSLCert       string
	SSLKey        string
}

type TorConfig struct {
	Password   string
	TorControl string
}

type IpnsExtraConfig struct {
	DHTQuorumSize int
	FallbackAPI   string
}

type WalletsConfig struct {
	BTC *CoinConfig `json:"BTC"`
	BCH *CoinConfig `json:"BCH"`
	LTC *CoinConfig `json:"LTC"`
	ZEC *CoinConfig `json:"ZEC"`
	ETH *CoinConfig `json:"ETH"`
}

type CoinConfig struct {
	Type             string                 `json:"Type"`
	APIPool          []string               `json:"API"`
	APITestnetPool   []string               `json:"APITestnet"`
	MaxFee           uint64                 `json:"MaxFee"`
	FeeAPI           string                 `json:"FeeAPI"`
	HighFeeDefault   uint64                 `json:"HighFeeDefault"`
	MediumFeeDefault uint64                 `json:"MediumFeeDefault"`
	LowFeeDefault    uint64                 `json:"LowFeeDefault"`
	TrustedPeer      string                 `json:"TrustedPeer"`
	WalletOptions    map[string]interface{} `json:"WalletOptions"`
}

type DataSharing struct {
	AcceptStoreRequests bool
	PushTo              []string
}

var MalformedConfigError = errors.New("config file is malformed")

func DefaultWalletsConfig() *WalletsConfig {
	var feeAPI = "https://btc.fees.openbazaar.org"
	return &WalletsConfig{
		BTC: &CoinConfig{
			Type:             WalletTypeAPI,
			APIPool:          CoinPoolBTC,
			APITestnetPool:   CoinPoolTBTC,
			FeeAPI:           feeAPI,
			LowFeeDefault:    1,
			MediumFeeDefault: 10,
			HighFeeDefault:   50,
			MaxFee:           200,
			WalletOptions:    nil,
		},
		BCH: &CoinConfig{
			Type:             WalletTypeAPI,
			APIPool:          CoinPoolBCH,
			APITestnetPool:   CoinPoolTBCH,
			FeeAPI:           "", // intentionally blank
			LowFeeDefault:    1,
			MediumFeeDefault: 5,
			HighFeeDefault:   10,
			MaxFee:           200,
			WalletOptions:    nil,
		},
		LTC: &CoinConfig{
			Type:             WalletTypeAPI,
			APIPool:          CoinPoolLTC,
			APITestnetPool:   CoinPoolTLTC,
			FeeAPI:           "", // intentionally blank
			LowFeeDefault:    5,
			MediumFeeDefault: 10,
			HighFeeDefault:   20,
			MaxFee:           200,
			WalletOptions:    nil,
		},
		ZEC: &CoinConfig{
			Type:             WalletTypeAPI,
			APIPool:          CoinPoolZEC,
			APITestnetPool:   CoinPoolTZEC,
			FeeAPI:           "", // intentionally blank
			LowFeeDefault:    5,
			MediumFeeDefault: 10,
			HighFeeDefault:   20,
			MaxFee:           200,
			WalletOptions:    nil,
		},
		ETH: &CoinConfig{
			Type:             WalletTypeAPI,
			APIPool:          CoinPoolETH,
			APITestnetPool:   CoinPoolTETH,
			FeeAPI:           "", // intentionally blank
			LowFeeDefault:    7,
			MediumFeeDefault: 15,
			HighFeeDefault:   30,
			MaxFee:           200,
			WalletOptions:    EthereumDefaultOptions(),
		},
	}
}

func GetAPIConfig(cfgBytes []byte) (*APIConfig, error) {
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, MalformedConfigError
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError
	}

	apiIface, ok := cfg["JSON-API"]
	if !ok {
		return nil, MalformedConfigError
	}

	api, ok := apiIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError
	}

	var headers map[string]interface{}
	h, ok := api["HTTPHeaders"]
	if h == nil || !ok {
		headers = nil
	} else {
		headers, ok = h.(map[string]interface{})
		if !ok {
			return nil, MalformedConfigError
		}
	}

	enabled, ok := api["Enabled"]
	if !ok {
		return nil, MalformedConfigError
	}
	enabledBool, ok := enabled.(bool)
	if !ok {
		return nil, MalformedConfigError
	}
	authenticated := api["Authenticated"]
	if !ok {
		return nil, MalformedConfigError
	}
	authenticatedBool, ok := authenticated.(bool)
	if !ok {
		return nil, MalformedConfigError
	}
	allowedIPs, ok := api["AllowedIPs"]
	if !ok {
		return nil, MalformedConfigError
	}
	allowedIPsIface, ok := allowedIPs.([]interface{})
	if !ok {
		return nil, MalformedConfigError
	}
	var allowedIPstrings []string
	for _, ip := range allowedIPsIface {
		ipStr, ok := ip.(string)
		if !ok {
			return nil, MalformedConfigError
		}
		allowedIPstrings = append(allowedIPstrings, ipStr)
	}

	username, ok := api["Username"]
	if !ok {
		return nil, MalformedConfigError
	}
	usernameStr, ok := username.(string)
	if !ok {
		return nil, MalformedConfigError
	}

	password, ok := api["Password"]
	if !ok {
		return nil, MalformedConfigError
	}
	passwordStr, ok := password.(string)
	if !ok {
		return nil, MalformedConfigError
	}

	c, ok := api["CORS"]
	var cors *string
	if c == nil || !ok {
		cors = nil
	} else {
		crs, ok := c.(string)
		if !ok {
			return nil, MalformedConfigError
		}
		cors = &crs
	}
	sslEnabled, ok := api["SSL"]
	if !ok {
		return nil, MalformedConfigError
	}
	sslEnabledBool, ok := sslEnabled.(bool)
	if !ok {
		return nil, MalformedConfigError
	}

	certFile, ok := api["SSLCert"]
	if !ok {
		return nil, MalformedConfigError
	}
	certFileStr, ok := certFile.(string)
	if !ok {
		return nil, MalformedConfigError
	}
	keyFile, ok := api["SSLKey"]
	if !ok {
		return nil, MalformedConfigError
	}
	keyFileStr, ok := keyFile.(string)
	if !ok {
		return nil, MalformedConfigError
	}

	apiConfig := &APIConfig{
		Authenticated: authenticatedBool,
		AllowedIPs:    allowedIPstrings,
		Username:      usernameStr,
		Password:      passwordStr,
		CORS:          cors,
		Enabled:       enabledBool,
		HTTPHeaders:   headers,
		SSL:           sslEnabledBool,
		SSLCert:       certFileStr,
		SSLKey:        keyFileStr,
	}

	return apiConfig, nil
}

func GetWalletsConfig(cfgBytes []byte) (*WalletsConfig, error) {
	var cfgIface map[string]interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, MalformedConfigError
	}

	walletIface, ok := cfgIface["Wallets"]
	if !ok {
		return nil, MalformedConfigError
	}

	b, err := json.Marshal(walletIface)
	if err != nil {
		return nil, err
	}
	wCfg := new(WalletsConfig)
	err = json.Unmarshal(b, wCfg)
	if err != nil {
		return nil, err
	}
	return wCfg, nil
}

func GetTorConfig(cfgBytes []byte) (*TorConfig, error) {
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, MalformedConfigError
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError
	}

	tcIface, ok := cfg["Tor-config"]
	if !ok {
		return nil, MalformedConfigError
	}
	tc, ok := tcIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError
	}

	pw, ok := tc["Password"]
	if !ok {
		return nil, MalformedConfigError
	}
	pwStr, ok := pw.(string)
	if !ok {
		return nil, MalformedConfigError
	}
	controlUrl, ok := tc["TorControl"]
	if !ok {
		return nil, MalformedConfigError
	}
	controlUrlStr, ok := controlUrl.(string)
	if !ok {
		return nil, MalformedConfigError
	}

	return &TorConfig{TorControl: controlUrlStr, Password: pwStr}, nil
}

func GetIPNSExtraConfig(cfgBytes []byte) (*IpnsExtraConfig, error) {
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, MalformedConfigError
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError
	}

	ieIface, ok := cfg["IpnsExtra"]
	if !ok {
		return nil, MalformedConfigError
	}
	ieCfg, ok := ieIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError
	}

	quorumSize, ok := ieCfg["DHTQuorumSize"]
	if !ok {
		return nil, MalformedConfigError
	}
	qsInt, ok := quorumSize.(float64)
	if !ok {
		return nil, MalformedConfigError
	}
	fallbackAPI, ok := ieCfg["FallbackAPI"]
	if !ok {
		return nil, MalformedConfigError
	}
	fallbackAPIStr, ok := fallbackAPI.(string)
	if !ok {
		return nil, MalformedConfigError
	}

	return &IpnsExtraConfig{int(qsInt), fallbackAPIStr}, nil
}

func GetDropboxApiToken(cfgBytes []byte) (string, error) {
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return "", MalformedConfigError
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return "", MalformedConfigError
	}

	token, ok := cfg["Dropbox-api-token"]
	if !ok {
		return "", MalformedConfigError
	}
	tokenStr, ok := token.(string)
	if !ok {
		return "", MalformedConfigError
	}

	return tokenStr, nil
}

func GetRepublishInterval(cfgBytes []byte) (time.Duration, error) {
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return time.Duration(0), MalformedConfigError
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return time.Duration(0), MalformedConfigError
	}

	interval, ok := cfg["RepublishInterval"]
	if !ok {
		return time.Duration(0), MalformedConfigError
	}
	intervalStr, ok := interval.(string)
	if !ok {
		return time.Duration(0), MalformedConfigError
	}
	if intervalStr == "" {
		return time.Duration(0), nil
	}
	d, err := time.ParseDuration(intervalStr)
	if err != nil {
		return d, err
	}
	return d, nil
}

func GetDataSharing(cfgBytes []byte) (*DataSharing, error) {
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, MalformedConfigError
	}

	dataSharing := new(DataSharing)

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return dataSharing, MalformedConfigError
	}

	dscfg, ok := cfg["DataSharing"]
	if !ok {
		return dataSharing, MalformedConfigError
	}
	ds, ok := dscfg.(map[string]interface{})
	if !ok {
		return dataSharing, MalformedConfigError
	}

	acceptcfg, ok := ds["AcceptStoreRequests"]
	if !ok {
		return dataSharing, MalformedConfigError
	}
	accept, ok := acceptcfg.(bool)
	if !ok {
		return dataSharing, MalformedConfigError
	}
	dataSharing.AcceptStoreRequests = accept

	pushcfg, ok := ds["PushTo"]
	if !ok {
		return dataSharing, MalformedConfigError
	}
	pushList, ok := pushcfg.([]interface{})
	if !ok {
		return dataSharing, MalformedConfigError
	}

	for _, nd := range pushList {
		ndStr, ok := nd.(string)
		if !ok {
			return dataSharing, MalformedConfigError
		}
		dataSharing.PushTo = append(dataSharing.PushTo, ndStr)
	}
	return dataSharing, nil
}

func GetTestnetBootstrapAddrs(cfgBytes []byte) ([]string, error) {
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, MalformedConfigError
	}

	var addrs []string

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return addrs, MalformedConfigError
	}

	bootstrap, ok := cfg["Bootstrap-testnet"]
	if !ok {
		return addrs, MalformedConfigError
	}
	addrList, ok := bootstrap.([]interface{})
	if !ok {
		return addrs, MalformedConfigError
	}

	for _, addr := range addrList {
		addrStr, ok := addr.(string)
		if !ok {
			return addrs, MalformedConfigError
		}
		addrs = append(addrs, addrStr)
	}

	return addrs, nil
}
