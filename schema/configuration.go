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

type ResolverConfig struct {
	Id  string `json:".id"`
	Eth string `json:".eth"`
}

type WalletConfig struct {
	Type             string
	Binary           string
	MaxFee           int
	FeeAPI           string
	HighFeeDefault   int
	MediumFeeDefault int
	LowFeeDefault    int
	TrustedPeer      string
}

type WalletsConfig struct {
	BTC CoinConfig `json:"BTC"`
	BCH CoinConfig `json:"BCH"`
	LTC CoinConfig `json:"LTC"`
	ZEC CoinConfig `json:"ZEC"`
}

type CoinConfig struct {
	Type             string
	API              string
	APITestnet       string
	MaxFee           int
	FeeAPI           string
	HighFeeDefault   int
	MediumFeeDefault int
	LowFeeDefault    int
}

type DataSharing struct {
	AcceptStoreRequests bool
	PushTo              []string
}

var MalformedConfigError error = errors.New("Config file is malformed")

func GetAPIConfig(cfgBytes []byte) (*APIConfig, error) {
	var cfgIface interface{}
	json.Unmarshal(cfgBytes, &cfgIface)

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

func GetWalletConfig(cfgBytes []byte) (*WalletConfig, error) {
	var cfgIface interface{}
	json.Unmarshal(cfgBytes, &cfgIface)
	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError
	}

	walletIface, ok := cfg["Wallet"]
	if !ok {
		return nil, MalformedConfigError
	}
	wallet, ok := walletIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError
	}
	feeAPI, ok := wallet["FeeAPI"]
	if !ok {
		return nil, MalformedConfigError
	}
	feeAPIstr, ok := feeAPI.(string)
	if !ok {
		return nil, MalformedConfigError
	}
	trustedPeer, ok := wallet["TrustedPeer"]
	if !ok {
		return nil, MalformedConfigError
	}
	trustedPeerStr, ok := trustedPeer.(string)
	if !ok {
		return nil, MalformedConfigError
	}
	low, ok := wallet["LowFeeDefault"]
	if !ok {
		return nil, MalformedConfigError
	}
	lowFloat, ok := low.(float64)
	if !ok {
		return nil, MalformedConfigError
	}
	medium, ok := wallet["MediumFeeDefault"]
	if !ok {
		return nil, MalformedConfigError
	}
	mediumFloat, ok := medium.(float64)
	if !ok {
		return nil, MalformedConfigError
	}
	high, ok := wallet["HighFeeDefault"]
	if !ok {
		return nil, MalformedConfigError
	}
	highFloat, ok := high.(float64)
	if !ok {
		return nil, MalformedConfigError
	}
	maxFee, ok := wallet["MaxFee"]
	if !ok {
		return nil, MalformedConfigError
	}
	maxFeeFloat, ok := maxFee.(float64)
	if !ok {
		return nil, MalformedConfigError
	}
	walletType, ok := wallet["Type"]
	if !ok {
		return nil, MalformedConfigError
	}
	walletTypeStr, ok := walletType.(string)
	if !ok {
		return nil, MalformedConfigError
	}
	binary, ok := wallet["Binary"]
	if !ok {
		return nil, MalformedConfigError
	}
	binaryStr, ok := binary.(string)
	if !ok {
		return nil, MalformedConfigError
	}
	wCfg := &WalletConfig{
		Type:             walletTypeStr,
		Binary:           binaryStr,
		MaxFee:           int(maxFeeFloat),
		FeeAPI:           feeAPIstr,
		HighFeeDefault:   int(highFloat),
		MediumFeeDefault: int(mediumFloat),
		LowFeeDefault:    int(lowFloat),
		TrustedPeer:      trustedPeerStr,
	}
	return wCfg, nil
}

func GetWalletsConfig(cfgBytes []byte) (*WalletsConfig, error) {
	var cfgIface interface{}
	json.Unmarshal(cfgBytes, &cfgIface)
	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError
	}

	walletIface, ok := cfg["Wallets"]
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
	json.Unmarshal(cfgBytes, &cfgIface)

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

func GetDropboxApiToken(cfgBytes []byte) (string, error) {
	var cfgIface interface{}
	json.Unmarshal(cfgBytes, &cfgIface)

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
	json.Unmarshal(cfgBytes, &cfgIface)

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
	json.Unmarshal(cfgBytes, &cfgIface)
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
	json.Unmarshal(cfgBytes, &cfgIface)
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

func GetResolverConfig(cfgBytes []byte) (*ResolverConfig, error) {
	var cfgIface interface{}
	json.Unmarshal(cfgBytes, &cfgIface)

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError
	}

	r, ok := cfg["Resolvers"]
	if !ok {
		return nil, MalformedConfigError
	}
	resolverMap, ok := r.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError
	}
	blockstack, ok := resolverMap[".id"]
	if !ok {
		return nil, MalformedConfigError
	}

	idStr, ok := blockstack.(string)
	if !ok {
		return nil, MalformedConfigError
	}

	resolvers := &ResolverConfig{
		Id: idStr,
	}

	return resolvers, nil
}
