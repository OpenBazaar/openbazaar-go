package libbitcoin

import (
	btc "github.com/btcsuite/btcutil"
	b32 "github.com/tyler-smith/go-bip32"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
)

// BIP32 hierarchy
// m / account' / change / address_index
// Only account 0 is used. Other accounts reserved for extensions.
// change 0: receiving address
// change 1: change address
// change 2: refund address

func (w *LibbitcoinWallet) GetMasterPrivateKey() *b32.Key {
	return w.masterPrivateKey
}

func (w *LibbitcoinWallet) GetMasterPublicKey() *b32.Key {
	return w.masterPublicKey
}

func (w *LibbitcoinWallet) GetCurrentAddress(purpose bitcoin.KeyPurpose) *btc.AddressPubKeyHash {
	key, _ := w.db.Keys().GetCurrentKey(purpose)
	if key == nil {
		// FIXME: Generate new key
		return nil
	} else {
		pubkey, _ := btc.NewAddressPubKey(key.PublicKey().Key, w.Params)
		return pubkey.AddressPubKeyHash()
	}

}

func (w *LibbitcoinWallet) GetFreshAddress(purpose bitcoin.KeyPurpose) *btc.AddressPubKeyHash {
	// TODO: issued keys need to be tracked in the db. Just returning key 0 for now.
	accountMK, _ := w.masterPrivateKey.NewChildKey(b32.FirstHardenedChild)
	changeMK, _ := accountMK.NewChildKey(uint32(purpose))
	childKey, _ := changeMK.NewChildKey(0)
	pubkey, _ := btc.NewAddressPubKey(childKey.PublicKey().Key, w.Params)
	return pubkey.AddressPubKeyHash()
}
