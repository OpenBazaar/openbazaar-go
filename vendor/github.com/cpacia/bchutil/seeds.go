package bchutil

import "github.com/btcsuite/btcd/chaincfg"

var MainnetDNSSeeds = []chaincfg.DNSSeed{
	{"seed.bitcoinabc.org", true},
	{"seed-abc.bitcoinforks.org", true},
	{"seed.bitcoinunlimited.info", true},
	{"seed.bitprim.org", true},
	{"seed.deadalnix.me", true},
}

var TestnetDNSSeeds = []chaincfg.DNSSeed{
	{"testnet-seed.bitcoinabc.org", true},
	{"testnet-seed-abc.bitcoinforks.org", true},
	{"testnet-seed.bitcoinunlimited.info", true},
	{"testnet-seed.bitprim.org", true},
	{"testnet-seed.deadalnix.me", true},
}

func GetDNSSeed(params *chaincfg.Params) []chaincfg.DNSSeed {
	if params.Name == chaincfg.MainNetParams.Name {
		return MainnetDNSSeeds
	}
	return TestnetDNSSeeds
}
