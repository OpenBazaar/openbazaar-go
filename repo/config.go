package repo

import (
	"github.com/ipfs/go-ipfs/repo"
	"encoding/json"
	"io/ioutil"
	"github.com/OpenBazaar/go-libbitcoinclient"
	"encoding/base64"
	"github.com/pebbe/zmq4"
)

func GetLibbitcoinServers(cfgPath string, testnet bool) ([]libbitcoin.Server, error) {
	servers := []libbitcoin.Server{}
	file, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return servers, err
	}
	var ls interface{}
	json.Unmarshal(file, &ls)
	var net string
	if testnet {
		net = "Testnet"
	} else {
		net = "Mainnet"
	}

	for _, s := range(ls.(map[string]interface{})["LibbitcoinServers"].(map[string]interface{})[net].([]interface{})){
		encodedKey := s.(map[string]interface{})["PublicKey"].(string)
		if encodedKey != "" {
			b, _ := base64.StdEncoding.DecodeString(s.(map[string]interface{})["PublicKey"].(string))
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

func extendConfigFile(r repo.Repo, key string, value interface{}) error {
	if err := r.SetConfigKey(key, value); err != nil {
		return err
	}
	if err := r.Close(); err != nil {
		return err
	}
	return nil
}