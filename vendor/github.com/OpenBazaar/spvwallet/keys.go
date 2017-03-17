package spvwallet

import (
	"github.com/btcsuite/btcd/txscript"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/btcsuite/goleveldb/leveldb/errors"
)

const LOOKAHEADWINDOW = 100

type KeyPath struct {
	Purpose KeyPurpose
	Index   int
}

func (ts *TxStore) GetCurrentKey(purpose KeyPurpose) (*hd.ExtendedKey, error) {
	i, err := ts.Keys().GetUnused(purpose)
	if err != nil {
		return nil, err
	}
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
	for {
		childKey, err = ts.generateChildKey(purpose, uint32(index))
		if err == nil {
			break
		}
		index += 1
	}
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
		k, err := ts.generateChildKey(path.Purpose, uint32(path.Index))
		if err != nil {
			continue
		}
		keys = append(keys, k)
	}
	return keys
}

func (ts *TxStore) GetKeyForScript(scriptPubKey []byte) (*hd.ExtendedKey, error) {
	keyPath, err := ts.Keys().GetPathForScript(scriptPubKey)
	if err != nil {
		key, err := ts.Keys().GetKeyForScript(scriptPubKey)
		if err != nil {
			return nil, err
		}
		hdKey := hd.NewExtendedKey(
			ts.Param.HDPrivateKeyID[:],
			key.Serialize(),
			make([]byte, 32),
			[]byte{0x00, 0x00, 0x00, 0x00},
			0,
			0,
			true)
		return hdKey, nil
	}
	return ts.generateChildKey(keyPath.Purpose, uint32(keyPath.Index))
}

func (ts *TxStore) generateChildKey(purpose KeyPurpose, index uint32) (*hd.ExtendedKey, error) {
	if purpose == EXTERNAL {
		return ts.externalKey.Child(index)
	} else if purpose == INTERNAL {
		return ts.internalKey.Child(index)
	}
	return nil, errors.New("Unknown key purpose")
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
