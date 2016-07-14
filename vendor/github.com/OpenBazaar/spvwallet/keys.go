package spvwallet

import (
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/txscript"
)

type KeyPurpose int

const (
	EXTERNAL = 0
	INTERNAL = 1
)

const LOOKAHEADWINDOW = 100

type KeyPath struct {
	Purpose KeyPurpose
	Index   int
}


func (t *TxStore) GetCurrentKey(purpose KeyPurpose) *hd.ExtendedKey {
	i, _ := t.db.Keys().GetUnused(purpose)
	return t.generateChildKey(purpose, uint32(i))
}

func (t *TxStore) GetFreshKey(purpose KeyPurpose) *hd.ExtendedKey {
	index, _, err := t.db.Keys().GetLastKeyIndex(purpose)
	var childKey *hd.ExtendedKey
	if err != nil {
		childKey = t.generateChildKey(purpose, 0)
	} else {
		childKey = t.generateChildKey(purpose, uint32(index + 1))
	}
	addr, _ := childKey.Address(t.Param)
	script, _ := txscript.PayToAddrScript(addr)
	p := KeyPath{KeyPurpose(purpose), index + 1}
	t.db.Keys().Put(script, p)
	return childKey
}

func (t *TxStore) GetKeys() []*hd.ExtendedKey {
	var keys []*hd.ExtendedKey
	keyPaths, err := t.db.Keys().GetAll()
	if err != nil {
		return keys
	}
	for _, path := range keyPaths {
		keys = append(keys, t.generateChildKey(path.Purpose, uint32(path.Index)))
	}
	return keys
}

func (t *TxStore) GetKeyForScript(scriptPubKey []byte) (*hd.ExtendedKey, error) {
	keyPath, err := t.db.Keys().GetPathForScript(scriptPubKey)
	if err != nil {
		return nil, err
	}
	return t.generateChildKey(keyPath.Purpose, uint32(keyPath.Index)), nil
}

func (t *TxStore) generateChildKey(purpose KeyPurpose, index uint32) *hd.ExtendedKey {
	accountMK, _ := t.masterPrivKey.Child(hd.HardenedKeyStart + 0)
	purposeMK, _ := accountMK.Child(uint32(purpose))
	childKey, _ := purposeMK.Child(index)
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
