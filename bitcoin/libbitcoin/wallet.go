package libbitcoin

import (
	b32 "github.com/tyler-smith/go-bip32"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/OpenBazaar/go-libbitcoinclient"
)

type LibbitcoinWallet struct {
	Client              *libbitcoin.LibbitcoinClient

	Params              *chaincfg.Params

	MasterPrivateKey    *b32.Key
	MasterPublicKey     *b32.Key
}

func NewLibbitcoinWallet(seed []byte, params *chaincfg.Params, servers []libbitcoin.Server) *LibbitcoinWallet {
	mk, _ := b32.NewMasterKey(seed)
	l := new(LibbitcoinWallet)
	l.MasterPrivateKey = mk
	l.MasterPublicKey = mk.PublicKey()
	l.Params = params
	//l.Client = libbitcoin.NewLibbitcoinClient(servers, params)
	return l
}

