package libbitcoin

import (
	b32 "github.com/tyler-smith/go-bip32"
)

type LibbitcoinWallet struct {
	MasterPrivateKey    *b32.Key
	MasterPublicKey     *b32.Key
}

func NewLibbitcoinWallet(seed []byte) *LibbitcoinWallet {
	mk, _ := b32.NewMasterKey(seed)
	l := new(LibbitcoinWallet)
	l.MasterPrivateKey = mk
	l.MasterPublicKey = mk.PublicKey()
	return l
}

