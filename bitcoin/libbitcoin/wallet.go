package libbitcoin

import (
	b32 "github.com/tyler-smith/go-bip32"
	"github.com/btcsuite/btcd/chaincfg"
)

type LibbitcoinWallet struct {
	Params              *chaincfg.Params

	MasterPrivateKey    *b32.Key
	MasterPublicKey     *b32.Key
}

func NewLibbitcoinWallet(seed []byte, params *chaincfg.Params) *LibbitcoinWallet {
	mk, _ := b32.NewMasterKey(seed)
	l := new(LibbitcoinWallet)
	l.MasterPrivateKey = mk
	l.MasterPublicKey = mk.PublicKey()
	l.Params = params
	return l
}

