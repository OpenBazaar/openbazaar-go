package libbitcoin

import (
	b32 "github.com/tyler-smith/go-bip32"
)

func (w *LibbitcoinWallet) GetMasterPrivateKey() *b32.Key {
	return w.MasterPrivateKey
}

func (w *LibbitcoinWallet) GetMasterPublicKey() *b32.Key {
	return w.MasterPublicKey
}