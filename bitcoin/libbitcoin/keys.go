package libbitcoin

import (
	btc "github.com/btcsuite/btcutil"
	b32 "github.com/tyler-smith/go-bip32"
)

// BIP32 hierarchy
// m / account' / change / address_index
// Only account 0 is used. Other accounts reserved for extensions.
// change 0: receiving address
// change 1: change address
// change 2: refund address

func (w *LibbitcoinWallet) GetMasterPrivateKey() *b32.Key {
	return w.MasterPrivateKey
}

func (w *LibbitcoinWallet) GetMasterPublicKey() *b32.Key {
	return w.MasterPublicKey
}

func (w *LibbitcoinWallet) GetNextRefundAddress() *btc.AddressPubKeyHash {
	// TODO: issued keys need to be tracked in the db. Just returning key 0 for now.
	accountMK, _ := w.MasterPrivateKey.NewChildKey(b32.FirstHardenedChild)
	refundMK, _ := accountMK.NewChildKey(2)
	refundKey, _ := refundMK.NewChildKey(0)
	pubkey, _ := btc.NewAddressPubKey(refundKey.PublicKey().Key, w.Params)
	return pubkey.AddressPubKeyHash()
}
