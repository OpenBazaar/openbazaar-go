package spvwallet

import (
	"github.com/btcsuite/btcd/txscript"
	hd "github.com/btcsuite/btcutil/hdkeychain"
)

const LOOKAHEADWINDOW = 100

type KeyPath struct {
	Purpose KeyPurpose
	Index   int
}

func (ts *TxStore) GetCurrentKey(purpose KeyPurpose) *hd.ExtendedKey {
	i, _ := ts.Keys().GetUnused(purpose)
	return ts.generateChildKey(purpose, uint32(i))
}

func (ts *TxStore) GetFreshKey(purpose KeyPurpose) *hd.ExtendedKey {
	index, _, err := ts.Keys().GetLastKeyIndex(purpose)
	var childKey *hd.ExtendedKey
	if err != nil {
		index = 0
	} else {
		index += 1
	}
	childKey = ts.generateChildKey(purpose, uint32(index))
	addr, _ := childKey.Address(ts.Param)
	script, _ := txscript.PayToAddrScript(addr)
	p := KeyPath{KeyPurpose(purpose), index}
	ts.Keys().Put(script, p)
	return childKey
}

func (ts *TxStore) GetKeys() []*hd.ExtendedKey {
	var keys []*hd.ExtendedKey
	keyPaths, err := ts.Keys().GetAll()
	if err != nil {
		return keys
	}
	for _, path := range keyPaths {
		keys = append(keys, ts.generateChildKey(path.Purpose, uint32(path.Index)))
	}
	return keys
}

func (ts *TxStore) GetKeyForScript(scriptPubKey []byte) (*hd.ExtendedKey, error) {
	keyPath, err := ts.Keys().GetPathForScript(scriptPubKey)
	if err != nil {
		return nil, err
	}
	return ts.generateChildKey(keyPath.Purpose, uint32(keyPath.Index)), nil
}

func (ts *TxStore) generateChildKey(purpose KeyPurpose, index uint32) *hd.ExtendedKey {
	accountMK, _ := ts.masterPrivKey.Child(hd.HardenedKeyStart + 0)
	purposeMK, _ := accountMK.Child(uint32(purpose))
	childKey, _ := purposeMK.Child(index)
	return childKey
}

func (ts *TxStore) lookahead() {
	lookaheadWindows := ts.Keys().GetLookaheadWindows()
	for purpose, size := range lookaheadWindows {
		if size < LOOKAHEADWINDOW {
			for i := 0; i < (LOOKAHEADWINDOW - size); i++ {
				ts.GetFreshKey(purpose)
			}
		}
	}
}
