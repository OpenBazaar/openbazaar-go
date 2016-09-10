package repo

import (
	"encoding/json"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/config"
	"io/ioutil"
	"path"
)

var DefaultBootstrapAddresses = []string{
	"/ip4/107.170.133.32/tcp/4001/ipfs/QmSwHSqtUi9GhHTegi8gA5n2fsjP7LcnxGQeedj8ScLi8q", // Le Marché Serpette
	"/ip4/139.59.174.197/tcp/4001/ipfs/Qmf6ZASu56X3iS9zBh5CQRDCLHttDY41637qn87gzSGybs", // Brixton-Village
	"/ip4/139.59.6.222/tcp/4001/ipfs/QmZAZYJ5MvqkdoTuaFaoeyHkHLd8muENfr9JTo7ikQZPSG",   // Johari
}

func GetAPIUsernameAndPw(cfgPath string) (username, password string, err error) {
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return "", "", err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	api := cfg.(map[string]interface{})["OB-API"]
	uname := api.(map[string]interface{})["Username"].(string)
	pw := api.(map[string]interface{})["Password"].(string)

	return uname, pw, nil
}

func GetAPIHeaders(cfgPath string) (map[string][]string, error) {
	headers := make(map[string][]string)
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return headers, err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	api := cfg.(map[string]interface{})["OB-API"]
	h := api.(map[string]interface{})["HTTPHeaders"]
	if h == nil {
		headers = nil
	} else {
		headers = h.(map[string][]string)
	}

	return headers, nil
}

func GetAPIEnabled(cfgPath string) (bool, error) {
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return false, err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	api := cfg.(map[string]interface{})["OB-API"]
	enabled := api.(map[string]interface{})["Enabled"].(bool)
	return enabled, nil
}

func GetAPICORS(cfgPath string) (bool, error) {
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return false, err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	api := cfg.(map[string]interface{})["OB-API"]
	cors := api.(map[string]interface{})["CORS"].(bool)
	return cors, nil
}

func GetFeeAPI(cfgPath string) (string, error) {
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	wallet := cfg.(map[string]interface{})["Wallet"]
	feeAPI := wallet.(map[string]interface{})["FeeAPI"].(string)
	return feeAPI, nil
}

func GetTrustedBitcoinPeer(cfgPath string) (string, error) {
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	wallet := cfg.(map[string]interface{})["Wallet"]
	feeAPI := wallet.(map[string]interface{})["TrustedPeer"].(string)
	return feeAPI, nil
}

func GetDefaultFees(cfgPath string) (Low uint64, Medium uint64, High uint64, err error) {
	file, err := ioutil.ReadFile(cfgPath)
	ret := uint64(0)
	if err != nil {
		return ret, ret, ret, err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	wallet := cfg.(map[string]interface{})["Wallet"]
	low := wallet.(map[string]interface{})["LowFeeDefault"].(float64)
	medium := wallet.(map[string]interface{})["MediumFeeDefault"].(float64)
	high := wallet.(map[string]interface{})["HighFeeDefault"].(float64)
	return uint64(low), uint64(medium), uint64(high), nil
}

func GetMaxFee(cfgPath string) (uint64, error) {
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return 0, err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	wallet := cfg.(map[string]interface{})["Wallet"]
	maxFee := wallet.(map[string]interface{})["MaxFee"].(float64)
	return uint64(maxFee), nil
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

		// setup the node's default addresses.
		// NOTE: two swarm listen addrs, one tcp, one utp.
		Addresses: config.Addresses{
			Swarm: []string{
				"/ip4/0.0.0.0/tcp/4001",
				"/ip6/::/tcp/4001",
			},
			API:     "",
			Gateway: "/ip4/127.0.0.1/tcp/8080",
		},

		Datastore: datastore,
		Bootstrap: config.BootstrapPeerStrings(bootstrapPeers),
		Discovery: config.Discovery{config.MDNS{
			Enabled:  false,
			Interval: 10,
		}},

		// setup the node mount points.
		Mounts: config.Mounts{
			IPFS: "/ipfs",
			IPNS: "/ipns",
		},

		Ipns: config.Ipns{
			ResolveCacheSize: 128,
			RecordLifetime:   "7d",
			RepublishPeriod:  "24h",
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
