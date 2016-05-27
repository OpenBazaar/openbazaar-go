package libbitcoin

import (
	b32 "github.com/tyler-smith/go-bip32"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/OpenBazaar/go-libbitcoinclient"
	"github.com/tyler-smith/go-bip39"
)

type LibbitcoinWallet struct {
	Client              *libbitcoin.LibbitcoinClient

	Params              *chaincfg.Params

	MasterPrivateKey    *b32.Key
	MasterPublicKey     *b32.Key
}

func NewLibbitcoinWallet(mnemonic string, params *chaincfg.Params, servers []libbitcoin.Server) *LibbitcoinWallet {
	seed := bip39.NewSeed(mnemonic, "")
	mk, _ := b32.NewMasterKey(seed)
	l := new(LibbitcoinWallet)
	l.MasterPrivateKey = mk
	l.MasterPublicKey = mk.PublicKey()
	l.Params = params
	l.Client = libbitcoin.NewLibbitcoinClient(servers, params)
	return l
}

