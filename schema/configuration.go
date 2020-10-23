package schema

import (
	"encoding/json"
	"fmt"
	"strings"
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
	APIRouter     string
}

type WalletsConfig struct {
	BTC *CoinConfig `json:"BTC"`
	BCH *CoinConfig `json:"BCH"`
	LTC *CoinConfig `json:"LTC"`
	ZEC *CoinConfig `json:"ZEC"`
	ETH *CoinConfig `json:"ETH"`
	FIL *CoinConfig `json:"FIL"`
}

type CoinConfig struct {
	Type               string                 `json:"Type"`
	APIPool            []string               `json:"API"`
	APITestnetPool     []string               `json:"APITestnet"`
	MaxFee             uint64                 `json:"MaxFee"`
	FeeAPI             string                 `json:"FeeAPI"`
	SuperLowFeeDefault uint64                 `json:"SuperLowFeeDefault"`
	HighFeeDefault     uint64                 `json:"HighFeeDefault"`
	MediumFeeDefault   uint64                 `json:"MediumFeeDefault"`
	LowFeeDefault      uint64                 `json:"LowFeeDefault"`
	TrustedPeer        string                 `json:"TrustedPeer"`
	WalletOptions      map[string]interface{} `json:"WalletOptions"`
}

type DataSharing struct {
	AcceptStoreRequests bool
	PushTo              []string
}

type malformedConfigError struct {
	path []string
}

func malformedConfigKey(pathArgs ...string) malformedConfigError {
	return malformedConfigError{path: pathArgs}
}

func (err malformedConfigError) Error() string {
	if len(err.path) != 0 {
		return fmt.Sprintf("malformed config: %s", strings.Join(err.path, "."))
	}
	return "malformed config"
}

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
		FIL: &CoinConfig{
			Type:             WalletTypeAPI,
			APIPool:          CoinPoolFIL,
			APITestnetPool:   CoinPoolTFIL,
			FeeAPI:           "", // intentionally blank
			LowFeeDefault:    7,
			MediumFeeDefault: 15,
			HighFeeDefault:   30,
			MaxFee:           200,
			WalletOptions:    nil,
		},
	}
}

func GetAPIConfig(cfgBytes []byte) (*APIConfig, error) {
	const (
		KeyAllowedIPs    = "AllowedIPs"
		KeyAuthenticated = "Authenticated"
		KeyCORS          = "CORS"
		KeyEnabled       = "Enabled"
		KeyHTTPHeaders   = "HTTPHeaders"
		KeyJSONAPI       = "JSON-API"
		KeyPassword      = "Password"
		KeySSL           = "SSL"
		KeySSLCert       = "SSLCert"
		KeySSLKey        = "SSLKey"
		KeyUsername      = "Username"
	)
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, malformedConfigError{}
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return nil, malformedConfigError{}
	}

	apiIface, ok := cfg[KeyJSONAPI]
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI)
	}

	api, ok := apiIface.(map[string]interface{})
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI)
	}

	var headers map[string]interface{}
	h, ok := api[KeyHTTPHeaders]
	if h == nil || !ok {
		headers = nil
	} else {
		headers, ok = h.(map[string]interface{})
		if !ok {
			return nil, malformedConfigKey(KeyJSONAPI, KeyHTTPHeaders)
		}
	}

	enabled, ok := api[KeyEnabled]
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeyEnabled)
	}
	enabledBool, ok := enabled.(bool)
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeyEnabled)
	}
	authenticated := api[KeyAuthenticated]
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeyAuthenticated)
	}
	authenticatedBool, ok := authenticated.(bool)
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeyAuthenticated)
	}
	allowedIPs, ok := api[KeyAllowedIPs]
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeyAllowedIPs)
	}
	allowedIPsIface, ok := allowedIPs.([]interface{})
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeyAllowedIPs)
	}
	var allowedIPstrings []string
	for _, ip := range allowedIPsIface {
		ipStr, ok := ip.(string)
		if !ok {
			return nil, malformedConfigKey(KeyJSONAPI, KeyAllowedIPs)
		}
		allowedIPstrings = append(allowedIPstrings, ipStr)
	}

	username, ok := api[KeyUsername]
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeyUsername)
	}
	usernameStr, ok := username.(string)
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeyUsername)
	}

	password, ok := api[KeyPassword]
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeyPassword)
	}
	passwordStr, ok := password.(string)
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeyPassword)
	}

	c, ok := api[KeyCORS]
	var cors *string
	if c == nil || !ok {
		cors = nil
	} else {
		crs, ok := c.(string)
		if !ok {
			return nil, malformedConfigKey(KeyJSONAPI, KeyCORS)
		}
		cors = &crs
	}
	sslEnabled, ok := api[KeySSL]
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeySSL)
	}
	sslEnabledBool, ok := sslEnabled.(bool)
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeySSL)
	}

	certFile, ok := api[KeySSLCert]
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeySSLCert)
	}
	certFileStr, ok := certFile.(string)
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeySSLCert)
	}
	keyFile, ok := api[KeySSLKey]
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeySSLKey)
	}
	keyFileStr, ok := keyFile.(string)
	if !ok {
		return nil, malformedConfigKey(KeyJSONAPI, KeySSLKey)
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
	const KeyWallets = "Wallets"
	var cfgIface map[string]interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, malformedConfigError{}
	}

	walletIface, ok := cfgIface[KeyWallets]
	if !ok {
		return nil, malformedConfigKey(KeyWallets)
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
	const (
		KeyPassword   = "Password"
		KeyTorConfig  = "Tor-config"
		KeyTorControl = "TorControl"
	)
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, malformedConfigError{}
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return nil, malformedConfigError{}
	}

	tcIface, ok := cfg[KeyTorConfig]
	if !ok {
		return nil, malformedConfigKey(KeyTorConfig)
	}
	tc, ok := tcIface.(map[string]interface{})
	if !ok {
		return nil, malformedConfigKey(KeyTorConfig)
	}

	pw, ok := tc[KeyPassword]
	if !ok {
		return nil, malformedConfigKey(KeyTorConfig, KeyPassword)
	}
	pwStr, ok := pw.(string)
	if !ok {
		return nil, malformedConfigKey(KeyTorConfig, KeyPassword)
	}
	controlUrl, ok := tc[KeyTorControl]
	if !ok {
		return nil, malformedConfigKey(KeyTorConfig, KeyTorControl)
	}
	controlUrlStr, ok := controlUrl.(string)
	if !ok {
		return nil, malformedConfigKey(KeyTorConfig, KeyTorControl)
	}

	return &TorConfig{TorControl: controlUrlStr, Password: pwStr}, nil
}

func GetIPNSExtraConfig(cfgBytes []byte) (*IpnsExtraConfig, error) {
	const (
		KeyAPIRouter     = "APIRouter"
		KeyDHTQuorumSize = "DHTQuorumSize"
		KeyIpnsExtra     = "IpnsExtra"
	)
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, malformedConfigError{}
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return nil, malformedConfigError{}
	}

	ieIface, ok := cfg[KeyIpnsExtra]
	if !ok {
		return nil, malformedConfigKey(KeyIpnsExtra)
	}
	ieCfg, ok := ieIface.(map[string]interface{})
	if !ok {
		return nil, malformedConfigKey(KeyIpnsExtra)
	}

	quorumSize, ok := ieCfg[KeyDHTQuorumSize]
	if !ok {
		return nil, malformedConfigKey(KeyIpnsExtra, KeyDHTQuorumSize)
	}
	qsInt, ok := quorumSize.(float64)
	if !ok {
		return nil, malformedConfigKey(KeyIpnsExtra, KeyDHTQuorumSize)
	}
	apiRouter, ok := ieCfg[KeyAPIRouter]
	if !ok {
		return nil, malformedConfigKey(KeyIpnsExtra, KeyAPIRouter)
	}
	apiRouterStr, ok := apiRouter.(string)
	if !ok {
		return nil, malformedConfigKey(KeyIpnsExtra, KeyAPIRouter)
	}

	return &IpnsExtraConfig{int(qsInt), apiRouterStr}, nil
}

func GetDropboxApiToken(cfgBytes []byte) (string, error) {
	const KeyDropboxApiToken = "Dropbox-api-token"
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return "", malformedConfigError{}
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return "", malformedConfigError{}
	}

	token, ok := cfg[KeyDropboxApiToken]
	if !ok {
		return "", malformedConfigKey(KeyDropboxApiToken)
	}
	tokenStr, ok := token.(string)
	if !ok {
		return "", malformedConfigKey(KeyDropboxApiToken)
	}

	return tokenStr, nil
}

func GetRepublishInterval(cfgBytes []byte) (time.Duration, error) {
	const KeyRepublishInterval = "RepublishInterval"
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return time.Duration(0), malformedConfigError{}
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return time.Duration(0), malformedConfigError{}
	}

	interval, ok := cfg[KeyRepublishInterval]
	if !ok {
		return time.Duration(0), malformedConfigKey(KeyRepublishInterval)
	}
	intervalStr, ok := interval.(string)
	if !ok {
		return time.Duration(0), malformedConfigKey(KeyRepublishInterval)
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
	const (
		KeyAcceptStoreRequest = "AcceptStoreRequests"
		KeyDataSharing        = "DataSharing"
		KeyPushTo             = "PushTo"
	)

	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, malformedConfigError{}
	}

	dataSharing := new(DataSharing)

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return dataSharing, malformedConfigError{}
	}

	dscfg, ok := cfg[KeyDataSharing]
	if !ok {
		return dataSharing, malformedConfigKey(KeyDataSharing)
	}
	ds, ok := dscfg.(map[string]interface{})
	if !ok {
		return dataSharing, malformedConfigKey(KeyDataSharing)
	}

	acceptcfg, ok := ds[KeyAcceptStoreRequest]
	if !ok {
		return dataSharing, malformedConfigKey(KeyDataSharing, KeyAcceptStoreRequest)
	}
	accept, ok := acceptcfg.(bool)
	if !ok {
		return dataSharing, malformedConfigKey(KeyDataSharing, KeyAcceptStoreRequest)
	}
	dataSharing.AcceptStoreRequests = accept

	pushcfg, ok := ds[KeyPushTo]
	if !ok {
		return dataSharing, malformedConfigKey(KeyDataSharing, KeyPushTo)
	}
	pushList, ok := pushcfg.([]interface{})
	if !ok {
		return dataSharing, malformedConfigKey(KeyDataSharing, KeyPushTo)
	}

	for _, nd := range pushList {
		ndStr, ok := nd.(string)
		if !ok {
			return dataSharing, malformedConfigKey(KeyDataSharing, KeyPushTo)
		}
		dataSharing.PushTo = append(dataSharing.PushTo, ndStr)
	}
	return dataSharing, nil
}

func GetTestnetBootstrapAddrs(cfgBytes []byte) ([]string, error) {
	const KeyBootstrapTestnet = "Bootstrap-testnet"
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, malformedConfigError{}
	}

	var addrs []string

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return addrs, malformedConfigError{}
	}

	bootstrap, ok := cfg[KeyBootstrapTestnet]
	if !ok {
		return addrs, malformedConfigKey(KeyBootstrapTestnet)
	}
	addrList, ok := bootstrap.([]interface{})
	if !ok {
		return addrs, malformedConfigKey(KeyBootstrapTestnet)
	}

	for _, addr := range addrList {
		addrStr, ok := addr.(string)
		if !ok {
			return addrs, malformedConfigKey(KeyBootstrapTestnet)
		}
		addrs = append(addrs, addrStr)
	}

	return addrs, nil
}
