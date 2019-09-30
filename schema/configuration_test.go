package schema

import (
	"reflect"
	"testing"
	"time"
)

func TestGetApiConfig(t *testing.T) {
	config, err := GetAPIConfig(configFixture())
	if config.Username != "TestUsername" {
		t.Error("Expected TestUsername, got ", config.Username)
	}
	if config.Password != "TestPassword" {
		t.Error("Expected TestPassword, got ", config.Password)
	}
	if !config.Authenticated {
		t.Error("Expected Authenticated = true")
	}
	if len(config.AllowedIPs) != 1 || config.AllowedIPs[0] != "127.0.0.1" {
		t.Error("Expected AllowedIPs = [127.0.0.1]")
	}
	if config.CORS == nil {
		t.Error("Cors is not set")
	}
	if reflect.ValueOf(config.HTTPHeaders).Kind() != reflect.Map {
		t.Error("Headers is not a map")
	}
	if !config.Enabled {
		t.Error("Enabled is not true")
	}
	if !config.SSL {
		t.Error("Expected SSL = true")
	}
	if config.SSLCert == "" {
		t.Error("Expected test SSL cert, got ", config.SSLCert)
	}
	if config.SSLKey == "" {
		t.Error("Expected test SSL key, got ", config.SSLKey)
	}
	if err != nil {
		t.Error("GetAPIAuthentication threw an unexpected error")
	}

	_, err = GetAPIConfig([]byte{})
	if err == nil {
		t.Error("GetAPIAuthentication didn`t throw an error")
	}
}

func TestGetWalletsConfig(t *testing.T) {
	config, err := GetWalletsConfig(configFixture())
	if err != nil {
		t.Errorf("GetWalletsConfig threw an unexpected error: %s", err.Error())
	}
	if config.BTC.FeeAPI != "https://btc.fees.openbazaar.org" {
		t.Error("FeeApi does not equal expected value")
	}
	if config.BTC.Type != "API" {
		t.Error("Type does not equal expected value")
	}
	if len(config.BTC.APIPool) == 0 || config.BTC.APIPool[0] != "https://btc.api.openbazaar.org/api" {
		t.Error("BTC APIPool does not equal expected value")
	}
	if len(config.BTC.APITestnetPool) == 0 || config.BTC.APITestnetPool[0] != "https://tbtc.api.openbazaar.org/api" {
		t.Error("BTC APITestnetPool does not equal expected value")
	}
	if config.BTC.LowFeeDefault != 1 {
		t.Error("Expected low to be 1, got ", config.BTC.LowFeeDefault)
	}
	if config.BTC.MediumFeeDefault != 10 {
		t.Error("Expected medium to be 10, got ", config.BTC.MediumFeeDefault)
	}
	if config.BTC.HighFeeDefault != 50 {
		t.Error("Expected high to be 50, got ", config.BTC.HighFeeDefault)
	}
	if config.BTC.MaxFee != 200 {
		t.Error("Expected maxFee to be 200, got ", config.BTC.MaxFee)
	}

	if config.BCH.Type != "API" {
		t.Error("Type does not equal expected value")
	}
	if len(config.BCH.APIPool) == 0 || config.BCH.APIPool[0] != "https://bch.api.openbazaar.org/api" {
		t.Error("BCH APIPool does not equal expected value")
	}
	if len(config.BCH.APITestnetPool) == 0 || config.BCH.APITestnetPool[0] != "https://tbch.api.openbazaar.org/api" {

		t.Error("BCH APITestnetPool does not equal expected value")
	}
	if config.BCH.LowFeeDefault != 1 {
		t.Error("Expected low to be 1, got ", config.BCH.LowFeeDefault)
	}
	if config.BCH.MediumFeeDefault != 5 {
		t.Error("Expected medium to be 5, got ", config.BCH.MediumFeeDefault)
	}
	if config.BCH.HighFeeDefault != 10 {
		t.Error("Expected high to be 10, got ", config.BCH.HighFeeDefault)
	}
	if config.BTC.MaxFee != 200 {
		t.Error("Expected maxFee to be 200, got ", config.BTC.MaxFee)
	}

	if config.LTC.Type != "API" {
		t.Error("Type does not equal expected value")
	}
	if len(config.LTC.APIPool) == 0 || config.LTC.APIPool[0] != "https://ltc.api.openbazaar.org/api" {
		t.Error("LTC APIPool does not equal expected value")
	}
	if len(config.LTC.APITestnetPool) == 0 || config.LTC.APITestnetPool[0] != "https://tltc.api.openbazaar.org/api" {
		t.Error("LTC APITestnetPool does not equal expected value")
	}
	if config.LTC.LowFeeDefault != 5 {
		t.Error("Expected low to be 5, got ", config.LTC.LowFeeDefault)
	}
	if config.LTC.MediumFeeDefault != 10 {
		t.Error("Expected medium to be 10, got ", config.LTC.MediumFeeDefault)
	}
	if config.LTC.HighFeeDefault != 20 {
		t.Error("Expected high to be 20, got ", config.LTC.HighFeeDefault)
	}
	if config.LTC.MaxFee != 200 {
		t.Error("Expected maxFee to be 200, got ", config.LTC.MaxFee)
	}

	if config.ZEC.Type != "API" {
		t.Error("Type does not equal expected value")
	}
	if len(config.ZEC.APIPool) == 0 || config.ZEC.APIPool[0] != "https://zec.api.openbazaar.org/api" {
		t.Error("ZEC APIPool does not equal expected value")
	}
	if len(config.ZEC.APITestnetPool) == 0 || config.ZEC.APITestnetPool[0] != "https://tzec.api.openbazaar.org/api" {
		t.Error("ZEC APITestnetPool does not equal expected value")
	}
	if config.ZEC.LowFeeDefault != 5 {
		t.Error("Expected low to be 5, got ", config.ZEC.LowFeeDefault)
	}
	if config.ZEC.MediumFeeDefault != 10 {
		t.Error("Expected medium to be 10, got ", config.ZEC.MediumFeeDefault)
	}
	if config.ZEC.HighFeeDefault != 20 {
		t.Error("Expected high to be 20, got ", config.ZEC.HighFeeDefault)
	}
	if config.LTC.MaxFee != 200 {
		t.Error("Expected maxFee to be 200, got ", config.LTC.MaxFee)
	}

	_, err = GetWalletsConfig([]byte{})
	if err == nil {
		t.Error("GetWalletsConfig didn't throw an error")
	}
}

func TestGetDropboxApiToken(t *testing.T) {
	dropboxApiToken, err := GetDropboxApiToken(configFixture())
	if dropboxApiToken != "dropbox123" {
		t.Error("dropboxApiToken does not equal expected value")
	}
	if err != nil {
		t.Error("GetDropboxApiToken threw an unexpected error")
	}

	dropboxApiToken, err = GetDropboxApiToken([]byte{})
	if dropboxApiToken != "" {
		t.Error("Expected empty string, got ", dropboxApiToken)
	}
	if err == nil {
		t.Error("GetDropboxApiToken didn't throw an error")
	}
}

func TestGetIPNSExtraConfig(t *testing.T) {
	ipnsConfig, err := GetIPNSExtraConfig(configFixture())
	if err != nil {
		t.Error(err)
	}
	if ipnsConfig.DHTQuorumSize != 1 {
		t.Error("GetIPNSExtraConfig returned incorrect DHTQuorumSize")
	}
	if ipnsConfig.APIRouter != "https://routing.api.openbazaar.org" {
		t.Error("GetIPNSExtraConfig returned incorrect APIRouter")
	}
}

func TestRepublishInterval(t *testing.T) {
	interval, err := GetRepublishInterval(configFixture())
	if interval != time.Hour*24 {
		t.Error("RepublishInterval does not equal expected value")
	}
	if err != nil {
		t.Error("RepublishInterval threw an unexpected error")
	}

	interval, err = GetRepublishInterval([]byte{})
	if interval != time.Second*0 {
		t.Error("Expected zero duration, got ", interval)
	}
	if err == nil {
		t.Error("GetRepublishInterval didn't throw an error")
	}
}

func configFixture() []byte {
	return []byte(`{
  "API": {
    "HTTPHeaders": null
  },
  "Addresses": {
    "API": "",
    "Announce": null,
    "Gateway": "/ip4/127.0.0.1/tcp/4002",
    "NoAnnounce": null,
    "Swarm": [
      "/ip4/0.0.0.0/tcp/4001",
      "/ip4/0.0.0.0/udp/4001/utp",
      "/ip6/::/tcp/4001",
      "/ip6/::/udp/4001/utp"
    ]
  },
  "Bootstrap": [
    "/ip4/107.170.133.32/tcp/4001/ipfs/QmboEn7ycZqb8sXH6wJunWE6d3mdT9iVD7XWDmCcKE9jZ5",
    "/ip4/139.59.174.197/tcp/4001/ipfs/QmZbLxbrPfGKjhFPwv9g7PkT5jL5DzQ8mF3iioByWMAprj",
    "/ip4/139.59.6.222/tcp/4001/ipfs/QmPZkv392E7VxumGSugQDEpfk6bHxfv271HTdVvdUu5Sod"
  ],
  "DataSharing": {
    "AcceptStoreRequests": false,
    "PushTo": [
      "QmZbLxbrPfGKjhFPwv9g7PkT5jL5DzQ8mF3iioByWMAprj",
      "QmPZkv392E7VxumGSugQDEpfk6bHxfv271HTdVvdUu5Sod"
    ]
  },
  "Datastore": {
    "BloomFilterSize": 0,
    "GCPeriod": "1h",
    "HashOnRead": false,
    "Spec": {
      "mounts": [
        {
          "child": {
            "path": "blocks",
            "shardFunc": "/repo/flatfs/shard/v1/next-to-last/2",
            "sync": true,
            "type": "flatfs"
          },
          "mountpoint": "/blocks",
          "prefix": "flatfs.datastore",
          "type": "measure"
        },
        {
          "child": {
            "compression": "none",
            "path": "datastore",
            "type": "levelds"
          },
          "mountpoint": "/",
          "prefix": "leveldb.datastore",
          "type": "measure"
        }
      ],
      "type": "mount"
    },
    "StorageGCWatermark": 90,
    "StorageMax": "10GB"
  },
  "Discovery": {
    "MDNS": {
      "Enabled": false,
      "Interval": 10
    }
  },
  "Dropbox-api-token": "dropbox123",
  "Experimental": {
    "FilestoreEnabled": false,
    "Libp2pStreamMounting": false,
    "ShardingEnabled": false
  },
  "Gateway": {
    "HTTPHeaders": null,
    "PathPrefixes": [],
    "RootRedirect": "",
    "Writable": false
  },
  "Identity": {
    "PeerID": "testID",
    "PrivKey": "testKey"
  },
  "Ipns": {
    "BackUpAPI": "",
    "QuerySize": 0,
    "RecordLifetime": "7d",
    "RepublishPeriod": "24h",
    "ResolveCacheSize": 128,
    "UsePersistentCache": true
  },
  "IpnsExtra": {
    "DHTQuorumSize": 1,
    "APIRouter": "https://routing.api.openbazaar.org"
  },
  "JSON-API": {
    "AllowedIPs": [
      "127.0.0.1"
    ],
    "Authenticated": true,
    "CORS": "*",
    "Enabled": true,
    "HTTPHeaders": null,
    "Password": "TestPassword",
    "SSL": true,
    "SSLCert": "/path/to/ssl.cert",
    "SSLKey": "/path/to/ssl.key",
    "Username": "TestUsername"
  },
  "Mounts": {
    "FuseAllowOther": false,
    "IPFS": "/ipfs",
    "IPNS": "/ipns"
  },
  "Reprovider": {
    "Interval": "",
    "Strategy": ""
  },
  "RepublishInterval": "24h",
  "SupernodeRouting": {
    "Servers": null
  },
  "Swarm": {
    "AddrFilters": null,
    "DisableBandwidthMetrics": false,
    "DisableNatPortMap": false,
    "DisableRelay": false,
    "EnableRelayHop": false
  },
  "Tour": {
    "Last": ""
  },
  "LegacyWallet": {
    "Binary": "/path/to/bitcoind",
    "FeeAPI": "https://btc.fees.openbazaar.org",
    "HighFeeDefault": 60,
    "LowFeeDefault": 20,
    "MaxFee": 2000,
    "MediumFeeDefault": 40,
    "RPCPassword": "password",
    "RPCUser": "username",
    "TrustedPeer": "127.0.0.1:8333",
    "Type": "spvwallet"
  },
  "Wallets": {
    "BTC": {
      "Type": "API",
      "API": [
        "https://btc.api.openbazaar.org/api"
      ],
      "APITestnet": [
        "https://tbtc.api.openbazaar.org/api"
      ],
      "MaxFee": 200,
      "FeeAPI": "https://btc.fees.openbazaar.org",
      "HighFeeDefault": 50,
      "MediumFeeDefault": 10,
      "LowFeeDefault": 1,
      "TrustedPeer": "",
      "WalletOptions": null
    },
    "BCH": {
      "Type": "API",
      "API": [
        "https://bch.api.openbazaar.org/api"
      ],
      "APITestnet": [
        "https://tbch.api.openbazaar.org/api"
      ],
      "MaxFee": 200,
      "FeeAPI": "https://btc.fees.openbazaar.org",
      "HighFeeDefault": 10,
      "MediumFeeDefault": 5,
      "LowFeeDefault": 1,
      "TrustedPeer": "",
      "WalletOptions": null
    },
    "LTC": {
      "Type": "API",
      "API": [
        "https://ltc.api.openbazaar.org/api"
      ],
      "APITestnet": [
        "https://tltc.api.openbazaar.org/api"
      ],
      "MaxFee": 200,
      "FeeAPI": "https://btc.fees.openbazaar.org",
      "HighFeeDefault": 20,
      "MediumFeeDefault": 10,
      "LowFeeDefault": 5,
      "TrustedPeer": "",
      "WalletOptions": null
    },
    "ZEC": {
      "Type": "API",
      "API": [
        "https://zec.api.openbazaar.org/api"
      ],
      "APITestnet": [
        "https://tzec.api.openbazaar.org/api"
      ],
      "MaxFee": 200,
      "FeeAPI": "https://btc.fees.openbazaar.org",
      "HighFeeDefault": 20,
      "MediumFeeDefault": 10,
      "LowFeeDefault": 5,
      "TrustedPeer": "",
      "WalletOptions": null
    },
    "ETH": {
      "Type": "API",
      "API": [
        "https://mainnet.infura.io"
      ],
      "APITestnet": [
        "https://rinkeby.infura.io"
      ],
      "MaxFee": 200,
      "FeeAPI": "https://btc.fees.openbazaar.org",
      "HighFeeDefault": 30,
      "MediumFeeDefault": 15,
      "LowFeeDefault": 7,
      "TrustedPeer": "",
      "WalletOptions": {
        "RegistryAddress": "0x5c69ccf91eab4ef80d9929b3c1b4d5bc03eb0981",
        "RinkebyRegistryAddress": "0x5cEF053c7b383f430FC4F4e1ea2F7D31d8e2D16C",
        "RopstenRegistryAddress": "0x403d907982474cdd51687b09a8968346159378f3"
      }
    }
  }
}`)
}
