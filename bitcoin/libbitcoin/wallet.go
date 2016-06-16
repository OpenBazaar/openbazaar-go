package libbitcoin

import (
	"github.com/OpenBazaar/go-libbitcoinclient"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/btcsuite/btcd/chaincfg"
	b32 "github.com/tyler-smith/go-bip32"
	b39 "github.com/tyler-smith/go-bip39"
)

type LibbitcoinWallet struct {
	Client *libbitcoin.LibbitcoinClient

	Params *chaincfg.Params

	masterPrivateKey *b32.Key
	masterPublicKey  *b32.Key

	db repo.Datastore
}

func NewLibbitcoinWallet(mnemonic string, params *chaincfg.Params, db repo.Datastore, servers []libbitcoin.Server) *LibbitcoinWallet {
	seed := b39.NewSeed(mnemonic, "")
	mk, _ := b32.NewMasterKey(seed)
	l := new(LibbitcoinWallet)
	l.masterPrivateKey = mk
	l.masterPublicKey = mk.PublicKey()
	l.Params = params
	l.Client = libbitcoin.NewLibbitcoinClient(servers, params)
	l.db = db
	return l
}
