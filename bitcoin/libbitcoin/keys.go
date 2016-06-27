package libbitcoin

import (
	btc "github.com/btcsuite/btcutil"
	b32 "github.com/tyler-smith/go-bip32"
	"github.com/btcsuite/btcd/txscript"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"encoding/binary"
)

// BIP32 hierarchy
// m / account' / purpose / address_index
// Only account 0 is used. Other accounts reserved for extensions.
// purpose 0: receiving address
// purpose 1: change address
// purpose 2: refund address

func (w *LibbitcoinWallet) GetMasterPrivateKey() *b32.Key {
	return w.masterPrivateKey
}

func (w *LibbitcoinWallet) GetMasterPublicKey() *b32.Key {
	return w.masterPublicKey
}

func (w *LibbitcoinWallet) GetCurrentKey(purpose bitcoin.KeyPurpose) *b32.Key {
	key, used, _ := w.db.Keys().GetLastKey(purpose)
	if key == nil { // No keys in this chain have been generated yet. Let's generate key 0.
		childKey := w.generateChildKey(purpose, 0)
		addr, _ := btc.NewAddressPubKey(childKey.PublicKey().Key, w.Params)
		script, _ := txscript.PayToAddrScript(addr.AddressPubKeyHash())
		w.db.Keys().Put(childKey, script, purpose)
		if purpose == bitcoin.RECEIVING || purpose == bitcoin.REFUND {
			w.SubscribeAddress(addr.AddressPubKeyHash())
		}
		return childKey
	} else if used { // The last key in the chain has been used. Let's generated a new key and save it in the db.
		index := binary.BigEndian.Uint32(key.ChildNumber)
		childKey := w.generateChildKey(purpose, index + 1)
		addr, _ := btc.NewAddressPubKey(childKey.PublicKey().Key, w.Params)
		script, _ := txscript.PayToAddrScript(addr.AddressPubKeyHash())
		w.db.Keys().Put(childKey, script, purpose)
		if purpose == bitcoin.RECEIVING || purpose == bitcoin.REFUND {
			w.SubscribeAddress(addr.AddressPubKeyHash())
		}
		return childKey
	} else { // The last key in the chain is unused so let's just return it.
		return key
	}
}

func (w *LibbitcoinWallet) GetFreshKey(purpose bitcoin.KeyPurpose) *b32.Key {
	key, _, _ := w.db.Keys().GetLastKey(purpose)
	index := binary.BigEndian.Uint32(key.ChildNumber)
	childKey := w.generateChildKey(purpose, index + 1)
	addr, _ := btc.NewAddressPubKey(childKey.PublicKey().Key, w.Params)
	script, _ := txscript.PayToAddrScript(addr.AddressPubKeyHash())
	w.db.Keys().Put(childKey, script, purpose)
	if purpose == bitcoin.RECEIVING || purpose == bitcoin.REFUND {
		w.SubscribeAddress(addr.AddressPubKeyHash())
	}
	return childKey
}

func (w *LibbitcoinWallet) GetCurrentAddress(purpose bitcoin.KeyPurpose) *btc.AddressPubKeyHash {
	key := w.GetCurrentKey(purpose)
	addr, _ := btc.NewAddressPubKey(key.PublicKey().Key, w.Params)
	return addr.AddressPubKeyHash()
}

func (w *LibbitcoinWallet) GetFreshAddress(purpose bitcoin.KeyPurpose) *btc.AddressPubKeyHash {
	key := w.GetFreshKey(purpose)
	addr, _ := btc.NewAddressPubKey(key.PublicKey().Key, w.Params)
	return addr.AddressPubKeyHash()
}

func (w *LibbitcoinWallet) generateChildKey(purpose bitcoin.KeyPurpose, index uint32) *b32.Key {
	accountMK, _ := w.masterPrivateKey.NewChildKey(b32.FirstHardenedChild)
	purposeMK, _ := accountMK.NewChildKey(uint32(purpose))
	childKey, _ := purposeMK.NewChildKey(index)
	return childKey
}