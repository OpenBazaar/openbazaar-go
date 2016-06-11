package repo

import (
	"io"
	"github.com/ipfs/go-ipfs/repo"
	"encoding/json"
	"io/ioutil"
	"github.com/OpenBazaar/go-libbitcoinclient"
	"encoding/base64"
	"github.com/pebbe/zmq4"
	"github.com/ipfs/go-ipfs/repo/config"
)

var DefaultBootstrapAddresses = []string{
	"/ip4/107.170.133.32/tcp/4001/ipfs/QmboEn7ycZqb8sXH6wJunWE6d3mdT9iVD7XWDmCcKE9jZ5",  // Le Marché Serpette
	"/ip4/139.59.174.197/tcp/4001/ipfs/QmWPRBSdhmhfWLUZapvfg6GCuyYyscgPnaKycYfWqJJcdB",  // Brixton-Village
	"/ip4/139.59.6.222/tcp/4001/ipfs/QmVW1nDx2rt8eff8nAB3L8MannsHFsidje6YC3EQvhcwZF",    // Johar

}

func GetLibbitcoinServers(cfgPath string) ([]libbitcoin.Server, error) {
	servers := []libbitcoin.Server{}
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return servers, err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	for _, s := range(cfg.(map[string]interface{})["LibbitcoinServers"].([]interface{})){
		encodedKey := s.(map[string]interface{})["PublicKey"].(string)
		if encodedKey != "" {
			b, _ := base64.StdEncoding.DecodeString(encodedKey)
			encodedKey = zmq4.Z85encode(string(b))
		}
		server := libbitcoin.Server{
			Url: s.(map[string]interface{})["Url"].(string),
			PublicKey: encodedKey,
		}
		servers = append(servers, server)
	}
	return  servers, nil
}

func GetDropboxApiToken(cfgPath string) (string, error) {
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}
	var cfg interface{}
	json.Unmarshal(file, &cfg)

	token := cfg.(map[string]interface{})["Dropbox-api-token"].(string)

	return  token, nil
}

func extendConfigFile(r repo.Repo, key string, value interface{}) error {
	if err := r.SetConfigKey(key, value); err != nil {
		return err
	}
	return nil
}

func initConfig(out io.Writer) (*config.Config, error) {

	bootstrapPeers, err := config.ParseBootstrapPeers(DefaultBootstrapAddresses)
	if err != nil {
		return nil, err
	}

	datastore, err := datastoreConfig()
	if err != nil {
		return nil, err
	}

	conf := &config.Config{

		// setup the node's default addresses.
		// NOTE: two swarm listen addrs, one tcp, one utp.
		Addresses: config.Addresses{
			Swarm: []string{
				"/ip4/0.0.0.0/tcp/4001",
				"/ip4/0.0.0.0/udp/4001/utp",
				"/ip6/::/tcp/4001",
				"/ip6/::/udp/4001/utp",
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
			RecordLifetime: "7d",
			RepublishPeriod: "24h",
		},

		Gateway: config.Gateway{
			RootRedirect: "",
			Writable:     false,
			PathPrefixes: []string{},
		},
	}

	return conf, nil
}

func datastoreConfig() (config.Datastore, error) {
	dspath, err := config.DataStorePath("")
	if err != nil {
		return config.Datastore{}, err
	}
	return config.Datastore{
		Path:               dspath,
		Type:               "leveldb",
		StorageMax:         "10GB",
		StorageGCWatermark: 90, // 90%
		GCPeriod:           "1h",
	}, nil
}