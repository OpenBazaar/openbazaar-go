package spvwallet

import (
	btc "github.com/btcsuite/btcutil"
	b32 "github.com/tyler-smith/go-bip32"
	"github.com/btcsuite/btcd/txscript"
	"encoding/binary"
)

type KeyPurpose int

const (
	EXTERNAL = 0
	INTERNAL = 1
)

const LOOKAHEADWINDOW = 100

func (t *TxStore) GetCurrentKey(purpose KeyPurpose) *b32.Key {
	key, _ := t.db.Keys().GetUnused(purpose)
	return key
}

func (t *TxStore) GetFreshKey(purpose KeyPurpose) *b32.Key {
	key, _, _ := t.db.Keys().GetLastKey(purpose)
	var childKey *		b32.Key
	if key == nil {
		childKey = t.generateChildKey(purpose, 0)
	} else {
		index := binary.BigEndian.Uint32(key.ChildNumber)
		childKey = t.generateChildKey(purpose, index + 1)
	}
	addr, _ := btc.NewAddressPubKey(childKey.PublicKey().Key, t.Param)
	script, _ := txscript.PayToAddrScript(addr.AddressPubKeyHash())
	t.db.Keys().Put(childKey, script, purpose)
	return childKey
}

func (t *TxStore) generateChildKey(purpose KeyPurpose, index uint32) *b32.Key {
	accountMK, _ := t.masterPrivKey.NewChildKey(b32.FirstHardenedChild)
	purposeMK, _ := accountMK.NewChildKey(uint32(purpose))
	childKey, _ := purposeMK.NewChildKey(index)
	return childKey
}

func (t *TxStore) lookahead() {
	lookaheadWindows := t.db.Keys().GetLookaheadWindows()
	for purpose, size := range lookaheadWindows {
		if size < LOOKAHEADWINDOW {
			for i:=0; i<(LOOKAHEADWINDOW-size); i++ {
				t.GetFreshKey(purpose)
			}
		}
	}
}
