package repo

import (
	"encoding/json"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/config"
	"io/ioutil"
	"path"
)

var DefaultBootstrapAddresses = []string{
	"/ip4/107.170.133.32/tcp/4001/ipfs/QmbY4yo9Eifg7DPjL7qK5JvNdiJaRAD7N76gVg4YoQsvgA", // Le Marché Serpette
	"/ip4/139.59.174.197/tcp/4001/ipfs/QmcCoBtYyduyurcLHRF14QhhA88YojJJpGFuMHoMZuU8sc", // Brixton-Village
	"/ip4/139.59.6.222/tcp/4001/ipfs/QmdzzGGc9xZq8w4z42vSHe32DZM7VXfDUFEUyfPvYNYhXE",   // Johari
	"/ip4/46.101.198.170/tcp/4001/ipfs/QmePWxsFT9wY3QuukgVDB7XZpqdKhrqJTHTXU7ECLDWJqX", // Duo Search
}

type APIConfig struct {
	Authenticated bool
	AllowedIPs    []string
	Username      string
	Password      string
	CORS          *string
	Enabled       bool
	HTTPHeaders   map[string][]string
	SSL           bool
	SSLCert       string
	SSLKey        string
}

type TorConfig struct {
	Password   string
	TorControl string
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
	RPCUser          string
	RPCPassword      string
}

func GetAPIConfig(cfgPath string) (*APIConfig, error) {
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	api := cfg.(map[string]interface{})["JSON-API"]
	headers := make(map[string][]string)
	h := api.(map[string]interface{})["HTTPHeaders"]
	if h == nil {
		headers = nil
	} else {
		headers = h.(map[string][]string)
	}
	enabled := api.(map[string]interface{})["Enabled"].(bool)
	authenticated := api.(map[string]interface{})["Authenticated"].(bool)
	allowedIPs := api.(map[string]interface{})["AllowedIPs"].([]interface{})
	var allowedIPstrings []string
	for _, ip := range allowedIPs {
		allowedIPstrings = append(allowedIPstrings, ip.(string))
	}
	username := api.(map[string]interface{})["Username"].(string)
	password := api.(map[string]interface{})["Password"].(string)
	c := api.(map[string]interface{})["CORS"]
	var cors *string
	if c == nil {
		cors = nil
	} else {
		crs := c.(string)
		cors = &crs
	}
	sslEnabled := api.(map[string]interface{})["SSL"].(bool)
	certFile := api.(map[string]interface{})["SSLCert"].(string)
	keyFile := api.(map[string]interface{})["SSLKey"].(string)

	apiConfig := &APIConfig{
		Authenticated: authenticated,
		AllowedIPs:    allowedIPstrings,
		Username:      username,
		Password:      password,
		CORS:          cors,
		Enabled:       enabled,
		HTTPHeaders:   headers,
		SSL:           sslEnabled,
		SSLCert:       certFile,
		SSLKey:        keyFile,
	}

	return apiConfig, nil
}

func GetWalletConfig(cfgPath string) (*WalletConfig, error) {
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	wallet := cfg.(map[string]interface{})["Wallet"]
	feeAPI := wallet.(map[string]interface{})["FeeAPI"].(string)
	trustedPeer := wallet.(map[string]interface{})["TrustedPeer"].(string)
	low := wallet.(map[string]interface{})["LowFeeDefault"].(float64)
	medium := wallet.(map[string]interface{})["MediumFeeDefault"].(float64)
	high := wallet.(map[string]interface{})["HighFeeDefault"].(float64)
	maxFee := wallet.(map[string]interface{})["MaxFee"].(float64)
	walletType := wallet.(map[string]interface{})["Type"].(string)
	binary := wallet.(map[string]interface{})["Binary"].(string)
	rpcUser := wallet.(map[string]interface{})["RPCUser"].(string)
	rpcPassword := wallet.(map[string]interface{})["RPCPassword"].(string)
	wCfg := &WalletConfig{
		Type:             walletType,
		Binary:           binary,
		MaxFee:           int(maxFee),
		FeeAPI:           feeAPI,
		HighFeeDefault:   int(high),
		MediumFeeDefault: int(medium),
		LowFeeDefault:    int(low),
		TrustedPeer:      trustedPeer,
		RPCUser:          rpcUser,
		RPCPassword:      rpcPassword,
	}
	return wCfg, nil
}

func GetTorConfig(cfgPath string) (TorConfig, error) {
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return TorConfig{}, err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	tc := cfg.(map[string]interface{})["Tor-config"]
	pw := tc.(map[string]interface{})["Password"].(string)
	controlUrl := tc.(map[string]interface{})["TorControl"].(string)

	return TorConfig{TorControl: controlUrl, Password: pw}, nil
}

func GetDropboxApiToken(cfgPath string) (string, error) {
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	token := cfg.(map[string]interface{})["Dropbox-api-token"].(string)

	return token, nil
}

func GetCrosspostGateway(cfgPath string) ([]string, error) {
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	gwys := cfg.(map[string]interface{})["Crosspost-gateways"].([]interface{})

	var urls []string
	for _, gw := range gwys {
		urls = append(urls, gw.(string))
	}

	return urls, nil
}

func GetResolverUrl(cfgPath string) (string, error) {
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	r := cfg.(map[string]interface{})["Resolver"].(string)

	return r, nil
}

func extendConfigFile(r repo.Repo, key string, value interface{}) error {
	if err := r.SetConfigKey(key, value); err != nil {
		return err
	}
	return nil
}

func InitConfig(repoRoot string) (*config.Config, error) {
	bootstrapPeers, err := config.ParseBootstrapPeers(DefaultBootstrapAddresses)
	if err != nil {
		return nil, err
	}

	datastore := datastoreConfig(repoRoot)

	conf := &config.Config{

		// Setup the node's default addresses.
		// NOTE: two swarm listen addrs, one TCP, one UTP.
		Addresses: config.Addresses{
			Swarm: []string{
				"/ip4/0.0.0.0/tcp/4001",
				"/ip6/::/tcp/4001",
				"/ip4/0.0.0.0/tcp/9005/ws",
				"/ip6/::/tcp/9005/ws",
			},
			API:     "",
			Gateway: "/ip4/127.0.0.1/tcp/4002",
		},

		Datastore: datastore,
		Bootstrap: config.BootstrapPeerStrings(bootstrapPeers),
		Discovery: config.Discovery{config.MDNS{
			Enabled:  false,
			Interval: 10,
		}},

		// Setup the node mount points
		Mounts: config.Mounts{
			IPFS: "/ipfs",
			IPNS: "/ipns",
		},

		Ipns: config.Ipns{
			ResolveCacheSize: 128,
			RecordLifetime:   "7d",
			RepublishPeriod:  "24h",
			QuerySize:        5,
		},

		Gateway: config.Gateway{
			RootRedirect: "",
			Writable:     false,
			PathPrefixes: []string{},
		},
	}

	return conf, nil
}

func datastoreConfig(repoRoot string) config.Datastore {
	dspath := path.Join(repoRoot, "datastore")
	return config.Datastore{
		Path:               dspath,
		Type:               "leveldb",
		StorageMax:         "10GB",
		StorageGCWatermark: 90, // 90%
		GCPeriod:           "1h",
	}
}
