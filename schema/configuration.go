package schema

import (
	"encoding/json"
	"fmt"
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

type MalformedConfigError struct {
	What string
}

var _ error = MalformedConfigError{}

func (err MalformedConfigError) Error() string {
	return fmt.Sprintf("config file is malformed: %s", err.What)
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
	}
}

func sectionError(section string) string {
	return fmt.Sprintf("Error parsing section '%s'", section)
}

func keyError(section string, key string) string {
	if section == "" {
		return fmt.Sprintf("Error parsing key '%s'", key)
	}
	return fmt.Sprintf("Error parsing key '%s' in section '%s'", key, section)
}

func GetAPIConfig(cfgBytes []byte) (*APIConfig, error) {
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, MalformedConfigError{What: "JSON error"}
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError{What: "Config file is empty"}
	}

	const JSONAPI string = "JSON-API"

	apiIface, ok := cfg[JSONAPI]
	if !ok {
		return nil, MalformedConfigError{What: sectionError(JSONAPI)}
	}

	api, ok := apiIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError{What: sectionError(JSONAPI)}
	}

	var headers map[string]interface{}
	h, ok := api["HTTPHeaders"]
	if h == nil || !ok {
		headers = nil
	} else {
		headers, ok = h.(map[string]interface{})
		if !ok {
			return nil, MalformedConfigError{What: keyError(JSONAPI, "HTTPHeaders")}
		}
	}

	enabled, ok := api["Enabled"]
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "Enabled")}
	}
	enabledBool, ok := enabled.(bool)
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "Enabled")}
	}
	authenticated := api["Authenticated"]
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "Authenticated")}
	}
	authenticatedBool, ok := authenticated.(bool)
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "Authenticated")}
	}
	allowedIPs, ok := api["AllowedIPs"]
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "AllowedIPs")}
	}
	allowedIPsIface, ok := allowedIPs.([]interface{})
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "AllowedIPs")}
	}
	var allowedIPstrings []string
	for _, ip := range allowedIPsIface {
		ipStr, ok := ip.(string)
		if !ok {
			return nil, MalformedConfigError{What: keyError(JSONAPI, "AllowedIPs")}
		}
		allowedIPstrings = append(allowedIPstrings, ipStr)
	}

	username, ok := api["Username"]
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "Username")}
	}
	usernameStr, ok := username.(string)
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "Username")}
	}

	password, ok := api["Password"]
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "Password")}
	}
	passwordStr, ok := password.(string)
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "Password")}
	}

	c, ok := api["CORS"]
	var cors *string
	if c == nil || !ok {
		cors = nil
	} else {
		crs, ok := c.(string)
		if !ok {
			return nil, MalformedConfigError{What: keyError(JSONAPI, "CORS")}
		}
		cors = &crs
	}
	sslEnabled, ok := api["SSL"]
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "SSL")}
	}
	sslEnabledBool, ok := sslEnabled.(bool)
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "SSL")}
	}

	certFile, ok := api["SSLCert"]
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "SSLCert")}
	}
	certFileStr, ok := certFile.(string)
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "SSLCert")}
	}
	keyFile, ok := api["SSLKey"]
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "SSLKey")}
	}
	keyFileStr, ok := keyFile.(string)
	if !ok {
		return nil, MalformedConfigError{What: keyError(JSONAPI, "SSLKey")}
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
		return nil, MalformedConfigError{What: "JSON error"}
	}

	walletIface, ok := cfgIface["Wallets"]
	if !ok {
		return nil, MalformedConfigError{What: sectionError("Wallets")}
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
		return nil, MalformedConfigError{What: "JSON error"}
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError{What: "Config file is empty"}
	}

	const TORCONFIG string = "Tor-config"
	tcIface, ok := cfg["Tor-config"]
	if !ok {
		return nil, MalformedConfigError{What: sectionError(TORCONFIG)}
	}
	tc, ok := tcIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError{What: sectionError(TORCONFIG)}
	}

	pw, ok := tc["Password"]
	if !ok {
		return nil, MalformedConfigError{What: keyError(TORCONFIG, "Password")}
	}
	pwStr, ok := pw.(string)
	if !ok {
		return nil, MalformedConfigError{What: keyError(TORCONFIG, "Password")}
	}
	controlUrl, ok := tc["TorControl"]
	if !ok {
		return nil, MalformedConfigError{What: keyError(TORCONFIG, "TorControl")}
	}
	controlUrlStr, ok := controlUrl.(string)
	if !ok {
		return nil, MalformedConfigError{What: keyError(TORCONFIG, "TorControl")}
	}

	return &TorConfig{TorControl: controlUrlStr, Password: pwStr}, nil
}

func GetIPNSExtraConfig(cfgBytes []byte) (*IpnsExtraConfig, error) {
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, MalformedConfigError{What: "JSON error"}
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError{What: "JSON error"}
	}

	const IPNSEXTRA string = "IpnsExtra"
	ieIface, ok := cfg[IPNSEXTRA]
	if !ok {
		return nil, MalformedConfigError{What: sectionError(IPNSEXTRA)}
	}
	ieCfg, ok := ieIface.(map[string]interface{})
	if !ok {
		return nil, MalformedConfigError{What: sectionError(IPNSEXTRA)}
	}

	quorumSize, ok := ieCfg["DHTQuorumSize"]
	if !ok {
		return nil, MalformedConfigError{What: keyError(IPNSEXTRA, "DHTQuorumSize")}
	}
	qsInt, ok := quorumSize.(float64)
	if !ok {
		return nil, MalformedConfigError{What: keyError(IPNSEXTRA, "DHTQuorumSize")}
	}
	fallbackAPI, ok := ieCfg["FallbackAPI"]
	if !ok {
		return nil, MalformedConfigError{What: keyError(IPNSEXTRA, "FallbackAPI")}
	}
	fallbackAPIStr, ok := fallbackAPI.(string)
	if !ok {
		return nil, MalformedConfigError{What: keyError(IPNSEXTRA, "FallbackAPI")}
	}

	return &IpnsExtraConfig{int(qsInt), fallbackAPIStr}, nil
}

func GetDropboxApiToken(cfgBytes []byte) (string, error) {
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return "", MalformedConfigError{What: "JSON error"}
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return "", MalformedConfigError{What: "Config file is empty"}
	}

	token, ok := cfg["Dropbox-api-token"]
	if !ok {
		return "", MalformedConfigError{What: keyError("", "Dropbox-api-token")}
	}
	tokenStr, ok := token.(string)
	if !ok {
		return "", MalformedConfigError{What: keyError("", "Dropbox-api-token")}
	}

	return tokenStr, nil
}

func GetRepublishInterval(cfgBytes []byte) (time.Duration, error) {
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return time.Duration(0), MalformedConfigError{What: "JSON error"}
	}

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return time.Duration(0), MalformedConfigError{What: "Config file is empty"}
	}

	interval, ok := cfg["RepublishInterval"]
	if !ok {
		return time.Duration(0), MalformedConfigError{What: keyError("", "RepublishInterval")}
	}
	intervalStr, ok := interval.(string)
	if !ok {
		return time.Duration(0), MalformedConfigError{What: keyError("", "RepublishInterval")}
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
		return nil, MalformedConfigError{What: "JSON error"}
	}

	dataSharing := new(DataSharing)

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return dataSharing, MalformedConfigError{What: "Config file is empty"}
	}

	const DATASHARING string = "DataSharing"
	dscfg, ok := cfg[DATASHARING]
	if !ok {
		return dataSharing, MalformedConfigError{What: sectionError(DATASHARING)}
	}
	ds, ok := dscfg.(map[string]interface{})
	if !ok {
		return dataSharing, MalformedConfigError{What: sectionError(DATASHARING)}
	}

	acceptcfg, ok := ds["AcceptStoreRequests"]
	if !ok {
		return dataSharing, MalformedConfigError{What: keyError(DATASHARING, "AcceptStoreRequests")}
	}
	accept, ok := acceptcfg.(bool)
	if !ok {
		return dataSharing, MalformedConfigError{What: keyError(DATASHARING, "AcceptStoreRequests")}
	}
	dataSharing.AcceptStoreRequests = accept

	pushcfg, ok := ds["PushTo"]
	if !ok {
		return dataSharing, MalformedConfigError{What: keyError(DATASHARING, "PushTo")}
	}
	pushList, ok := pushcfg.([]interface{})
	if !ok {
		return dataSharing, MalformedConfigError{What: keyError(DATASHARING, "PushTo")}
	}

	for _, nd := range pushList {
		ndStr, ok := nd.(string)
		if !ok {
			return dataSharing, MalformedConfigError{What: keyError(DATASHARING, "PushTo")}
		}
		dataSharing.PushTo = append(dataSharing.PushTo, ndStr)
	}
	return dataSharing, nil
}

func GetTestnetBootstrapAddrs(cfgBytes []byte) ([]string, error) {
	var cfgIface interface{}
	err := json.Unmarshal(cfgBytes, &cfgIface)
	if err != nil {
		return nil, MalformedConfigError{What: "JSON error"}
	}

	var addrs []string

	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return addrs, MalformedConfigError{What: "Config file is empty"}
	}

	bootstrap, ok := cfg["Bootstrap-testnet"]
	if !ok {
		return addrs, MalformedConfigError{What: keyError("", "Bootstrap-testnet")}
	}
	addrList, ok := bootstrap.([]interface{})
	if !ok {
		return addrs, MalformedConfigError{What: keyError("", "Bootstrap-testnet")}
	}

	for _, addr := range addrList {
		addrStr, ok := addr.(string)
		if !ok {
			return addrs, MalformedConfigError{What: keyError("", "Bootstrap-testnet")}
		}
		addrs = append(addrs, addrStr)
	}

	return addrs, nil
}
