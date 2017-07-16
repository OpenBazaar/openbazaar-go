package spvwallet

import (
	"github.com/btcsuite/btcd/chaincfg"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/btcsuite/goleveldb/leveldb/errors"
)

const LOOKAHEADWINDOW = 100

type KeyPath struct {
	Purpose KeyPurpose
	Index   int
}

type KeyManager struct {
	datastore Keys
	params    *chaincfg.Params

	internalKey *hd.ExtendedKey
	externalKey *hd.ExtendedKey
}

func NewKeyManager(db Keys, params *chaincfg.Params, masterPrivKey *hd.ExtendedKey) (*KeyManager, error) {
	internal, external, err := Bip44Derivation(masterPrivKey)
	if err != nil {
		return nil, err
	}
	km := &KeyManager{
		datastore:   db,
		params:      params,
		internalKey: internal,
		externalKey: external,
	}
	if err := km.lookahead(); err != nil {
		return nil, err
	}
	return km, nil
}

// m / purpose' / coin_type' / account' / change / address_index
func Bip44Derivation(masterPrivKey *hd.ExtendedKey) (internal, external *hd.ExtendedKey, err error) {
	// Purpose = bip44
	fourtyFour, err := masterPrivKey.Child(hd.HardenedKeyStart + 44)
	if err != nil {
		return nil, nil, err
	}
	// Cointype = bitcoin
	bitcoin, err := fourtyFour.Child(hd.HardenedKeyStart + 0)
	if err != nil {
		return nil, nil, err
	}
	// Account = 0
	account, err := bitcoin.Child(hd.HardenedKeyStart + 0)
	if err != nil {
		return nil, nil, err
	}
	// Change(0) = external
	external, err = account.Child(0)
	if err != nil {
		return nil, nil, err
	}
	// Change(1) = internal
	internal, err = account.Child(1)
	if err != nil {
		return nil, nil, err
	}
	return internal, external, nil
}

func (km *KeyManager) GetCurrentKey(purpose KeyPurpose) (*hd.ExtendedKey, error) {
	i, err := km.datastore.GetUnused(purpose)
	if err != nil {
		return nil, err
	}
	if len(i) == 0 {
		return nil, errors.New("No unused keys in database")
	}
	return km.generateChildKey(purpose, uint32(i[0]))
}

func (km *KeyManager) GetFreshKey(purpose KeyPurpose) (*hd.ExtendedKey, error) {
	index, _, err := km.datastore.GetLastKeyIndex(purpose)
	var childKey *hd.ExtendedKey
	if err != nil {
		index = 0
	} else {
		index += 1
	}
	for {
		// There is a small possibility bip32 keys can be invalid. The procedure in such cases
		// is to discard the key and derive the next one. This loop will continue until a valid key
		// is derived.
		childKey, err = km.generateChildKey(purpose, uint32(index))
		if err == nil {
			break
		}
		index += 1
	}
	addr, err := childKey.Address(km.params)
	if err != nil {
		return nil, err
	}
	p := KeyPath{KeyPurpose(purpose), index}
	err = km.datastore.Put(addr.ScriptAddress(), p)
	if err != nil {
		return nil, err
	}
	return childKey, nil
}

func (km *KeyManager) GetKeys() []*hd.ExtendedKey {
	var keys []*hd.ExtendedKey
	keyPaths, err := km.datastore.GetAll()
	if err != nil {
		return keys
	}
	for _, path := range keyPaths {
		k, err := km.generateChildKey(path.Purpose, uint32(path.Index))
		if err != nil {
			continue
		}
		keys = append(keys, k)
	}
	return keys
}

func (km *KeyManager) GetKeyForScript(scriptAddress []byte) (*hd.ExtendedKey, error) {
	keyPath, err := km.datastore.GetPathForKey(scriptAddress)
	if err != nil {
		key, err := km.datastore.GetKey(scriptAddress)
		if err != nil {
			return nil, err
		}
		hdKey := hd.NewExtendedKey(
			km.params.HDPrivateKeyID[:],
			key.Serialize(),
			make([]byte, 32),
			[]byte{0x00, 0x00, 0x00, 0x00},
			0,
			0,
			true)
		return hdKey, nil
	}
	return km.generateChildKey(keyPath.Purpose, uint32(keyPath.Index))
}

// Mark the given key as used and extend the lookahead window
func (km *KeyManager) MarkKeyAsUsed(scriptAddress []byte) error {
	if err := km.datastore.MarkKeyAsUsed(scriptAddress); err != nil {
		return err
	}
	return km.lookahead()
}

func (km *KeyManager) generateChildKey(purpose KeyPurpose, index uint32) (*hd.ExtendedKey, error) {
	if purpose == EXTERNAL {
		return km.externalKey.Child(index)
	} else if purpose == INTERNAL {
		return km.internalKey.Child(index)
	}
	return nil, errors.New("Unknown key purpose")
}

func (km *KeyManager) lookahead() error {
	lookaheadWindows := km.datastore.GetLookaheadWindows()
	for purpose, size := range lookaheadWindows {
		if size < LOOKAHEADWINDOW {
			for i := 0; i < (LOOKAHEADWINDOW - size); i++ {
				_, err := km.GetFreshKey(purpose)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
