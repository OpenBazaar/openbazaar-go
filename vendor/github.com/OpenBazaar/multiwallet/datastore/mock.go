package datastore

import (
	"bytes"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

type MockDatastore struct {
	keys           wallet.Keys
	utxos          wallet.Utxos
	stxos          wallet.Stxos
	txns           wallet.Txns
	watchedScripts wallet.WatchedScripts
}

type MockMultiwalletDatastore struct {
	db map[wallet.CoinType]wallet.Datastore
	sync.Mutex
}

func (m *MockMultiwalletDatastore) GetDatastoreForWallet(coinType wallet.CoinType) (wallet.Datastore, error) {
	m.Lock()
	defer m.Unlock()
	db, ok := m.db[coinType]
	if !ok {
		return nil, errors.New("Cointype not supported")
	}
	return db, nil
}

func NewMockMultiwalletDatastore() *MockMultiwalletDatastore {
	db := make(map[wallet.CoinType]wallet.Datastore)
	db[wallet.Bitcoin] = wallet.Datastore(&MockDatastore{
		&MockKeyStore{Keys: make(map[string]*KeyStoreEntry)},
		&MockUtxoStore{utxos: make(map[string]*wallet.Utxo)},
		&MockStxoStore{stxos: make(map[string]*wallet.Stxo)},
		&MockTxnStore{txns: make(map[string]*txnStoreEntry)},
		&MockWatchedScriptsStore{scripts: make(map[string][]byte)},
	})
	db[wallet.BitcoinCash] = wallet.Datastore(&MockDatastore{
		&MockKeyStore{Keys: make(map[string]*KeyStoreEntry)},
		&MockUtxoStore{utxos: make(map[string]*wallet.Utxo)},
		&MockStxoStore{stxos: make(map[string]*wallet.Stxo)},
		&MockTxnStore{txns: make(map[string]*txnStoreEntry)},
		&MockWatchedScriptsStore{scripts: make(map[string][]byte)},
	})
	db[wallet.Zcash] = wallet.Datastore(&MockDatastore{
		&MockKeyStore{Keys: make(map[string]*KeyStoreEntry)},
		&MockUtxoStore{utxos: make(map[string]*wallet.Utxo)},
		&MockStxoStore{stxos: make(map[string]*wallet.Stxo)},
		&MockTxnStore{txns: make(map[string]*txnStoreEntry)},
		&MockWatchedScriptsStore{scripts: make(map[string][]byte)},
	})
	db[wallet.Litecoin] = wallet.Datastore(&MockDatastore{
		&MockKeyStore{Keys: make(map[string]*KeyStoreEntry)},
		&MockUtxoStore{utxos: make(map[string]*wallet.Utxo)},
		&MockStxoStore{stxos: make(map[string]*wallet.Stxo)},
		&MockTxnStore{txns: make(map[string]*txnStoreEntry)},
		&MockWatchedScriptsStore{scripts: make(map[string][]byte)},
	})
	db[wallet.Ethereum] = wallet.Datastore(&MockDatastore{
		&MockKeyStore{Keys: make(map[string]*KeyStoreEntry)},
		&MockUtxoStore{utxos: make(map[string]*wallet.Utxo)},
		&MockStxoStore{stxos: make(map[string]*wallet.Stxo)},
		&MockTxnStore{txns: make(map[string]*txnStoreEntry)},
		&MockWatchedScriptsStore{scripts: make(map[string][]byte)},
	})
	return &MockMultiwalletDatastore{db: db}
}

func (m *MockDatastore) Keys() wallet.Keys {
	return m.keys
}

func (m *MockDatastore) Utxos() wallet.Utxos {
	return m.utxos
}

func (m *MockDatastore) Stxos() wallet.Stxos {
	return m.stxos
}

func (m *MockDatastore) Txns() wallet.Txns {
	return m.txns
}

func (m *MockDatastore) WatchedScripts() wallet.WatchedScripts {
	return m.watchedScripts
}

type KeyStoreEntry struct {
	ScriptAddress []byte
	Path          wallet.KeyPath
	Used          bool
	Key           *btcec.PrivateKey
}

type MockKeyStore struct {
	Keys map[string]*KeyStoreEntry
	sync.Mutex
}

func (m *MockKeyStore) Put(scriptAddress []byte, keyPath wallet.KeyPath) error {
	m.Lock()
	defer m.Unlock()
	m.Keys[hex.EncodeToString(scriptAddress)] = &KeyStoreEntry{scriptAddress, keyPath, false, nil}
	return nil
}

func (m *MockKeyStore) ImportKey(scriptAddress []byte, key *btcec.PrivateKey) error {
	m.Lock()
	defer m.Unlock()
	kp := wallet.KeyPath{Purpose: wallet.EXTERNAL, Index: -1}
	m.Keys[hex.EncodeToString(scriptAddress)] = &KeyStoreEntry{scriptAddress, kp, false, key}
	return nil
}

func (m *MockKeyStore) MarkKeyAsUsed(scriptAddress []byte) error {
	m.Lock()
	defer m.Unlock()
	key, ok := m.Keys[hex.EncodeToString(scriptAddress)]
	if !ok {
		return errors.New("key does not exist")
	}
	key.Used = true
	return nil
}

func (m *MockKeyStore) GetLastKeyIndex(purpose wallet.KeyPurpose) (int, bool, error) {
	m.Lock()
	defer m.Unlock()
	i := -1
	used := false
	for _, key := range m.Keys {
		if key.Path.Purpose == purpose && key.Path.Index > i {
			i = key.Path.Index
			used = key.Used
		}
	}
	if i == -1 {
		return i, used, errors.New("No saved keys")
	}
	return i, used, nil
}

func (m *MockKeyStore) GetPathForKey(scriptAddress []byte) (wallet.KeyPath, error) {
	m.Lock()
	defer m.Unlock()
	key, ok := m.Keys[hex.EncodeToString(scriptAddress)]
	if !ok || key.Path.Index == -1 {
		return wallet.KeyPath{}, errors.New("key does not exist")
	}
	return key.Path, nil
}

func (m *MockKeyStore) GetKey(scriptAddress []byte) (*btcec.PrivateKey, error) {
	m.Lock()
	defer m.Unlock()
	for _, k := range m.Keys {
		if k.Path.Index == -1 && bytes.Equal(scriptAddress, k.ScriptAddress) {
			return k.Key, nil
		}
	}
	return nil, errors.New("Not found")
}

func (m *MockKeyStore) GetImported() ([]*btcec.PrivateKey, error) {
	m.Lock()
	defer m.Unlock()
	var keys []*btcec.PrivateKey
	for _, k := range m.Keys {
		if k.Path.Index == -1 {
			keys = append(keys, k.Key)
		}
	}
	return keys, nil
}

func (m *MockKeyStore) GetUnused(purpose wallet.KeyPurpose) ([]int, error) {
	m.Lock()
	defer m.Unlock()
	var i []int
	for _, key := range m.Keys {
		if !key.Used && key.Path.Purpose == purpose {
			i = append(i, key.Path.Index)
		}
	}
	sort.Ints(i)
	return i, nil
}

func (m *MockKeyStore) GetAll() ([]wallet.KeyPath, error) {
	m.Lock()
	defer m.Unlock()
	var kp []wallet.KeyPath
	for _, key := range m.Keys {
		kp = append(kp, key.Path)
	}
	return kp, nil
}

func (m *MockKeyStore) GetLookaheadWindows() map[wallet.KeyPurpose]int {
	m.Lock()
	defer m.Unlock()
	internalLastUsed := -1
	externalLastUsed := -1
	for _, key := range m.Keys {
		if key.Path.Purpose == wallet.INTERNAL && key.Used && key.Path.Index > internalLastUsed {
			internalLastUsed = key.Path.Index
		}
		if key.Path.Purpose == wallet.EXTERNAL && key.Used && key.Path.Index > externalLastUsed {
			externalLastUsed = key.Path.Index
		}
	}
	internalUnused := 0
	externalUnused := 0
	for _, key := range m.Keys {
		if key.Path.Purpose == wallet.INTERNAL && !key.Used && key.Path.Index > internalLastUsed {
			internalUnused++
		}
		if key.Path.Purpose == wallet.EXTERNAL && !key.Used && key.Path.Index > externalLastUsed {
			externalUnused++
		}
	}
	mp := make(map[wallet.KeyPurpose]int)
	mp[wallet.INTERNAL] = internalUnused
	mp[wallet.EXTERNAL] = externalUnused
	return mp
}

type MockUtxoStore struct {
	utxos map[string]*wallet.Utxo
	sync.Mutex
}

func (m *MockUtxoStore) Put(utxo wallet.Utxo) error {
	m.Lock()
	defer m.Unlock()
	key := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	m.utxos[key] = &utxo
	return nil
}

func (m *MockUtxoStore) GetAll() ([]wallet.Utxo, error) {
	m.Lock()
	defer m.Unlock()
	var utxos []wallet.Utxo
	for _, v := range m.utxos {
		utxos = append(utxos, *v)
	}
	return utxos, nil
}

func (m *MockUtxoStore) SetWatchOnly(utxo wallet.Utxo) error {
	m.Lock()
	defer m.Unlock()
	key := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	u, ok := m.utxos[key]
	if !ok {
		return errors.New("Not found")
	}
	u.WatchOnly = true
	return nil
}

func (m *MockUtxoStore) Delete(utxo wallet.Utxo) error {
	m.Lock()
	defer m.Unlock()
	key := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	_, ok := m.utxos[key]
	if !ok {
		return errors.New("Not found")
	}
	delete(m.utxos, key)
	return nil
}

type MockStxoStore struct {
	stxos map[string]*wallet.Stxo
	sync.Mutex
}

func (m *MockStxoStore) Put(stxo wallet.Stxo) error {
	m.Lock()
	defer m.Unlock()
	m.stxos[stxo.SpendTxid.String()] = &stxo
	return nil
}

func (m *MockStxoStore) GetAll() ([]wallet.Stxo, error) {
	m.Lock()
	defer m.Unlock()
	var stxos []wallet.Stxo
	for _, v := range m.stxos {
		stxos = append(stxos, *v)
	}
	return stxos, nil
}

func (m *MockStxoStore) Delete(stxo wallet.Stxo) error {
	m.Lock()
	defer m.Unlock()
	_, ok := m.stxos[stxo.SpendTxid.String()]
	if !ok {
		return errors.New("Not found")
	}
	delete(m.stxos, stxo.SpendTxid.String())
	return nil
}

type txnStoreEntry struct {
	txn       []byte
	value     int
	height    int
	timestamp time.Time
	watchOnly bool
}

type MockTxnStore struct {
	txns map[string]*txnStoreEntry
	sync.Mutex
}

func (m *MockTxnStore) Put(tx []byte, txid string, value, height int, timestamp time.Time, watchOnly bool) error {
	m.Lock()
	defer m.Unlock()
	m.txns[txid] = &txnStoreEntry{
		txn:       tx,
		value:     value,
		height:    height,
		timestamp: timestamp,
		watchOnly: watchOnly,
	}
	return nil
}

func (m *MockTxnStore) Get(txid chainhash.Hash) (wallet.Txn, error) {
	m.Lock()
	defer m.Unlock()
	t, ok := m.txns[txid.String()]
	if !ok {
		return wallet.Txn{}, errors.New("Not found")
	}
	return wallet.Txn{
		Txid:      txid.String(),
		Value:     int64(t.value),
		Height:    int32(t.height),
		Timestamp: t.timestamp,
		WatchOnly: t.watchOnly,
		Bytes:     t.txn,
	}, nil
}

func (m *MockTxnStore) GetAll(includeWatchOnly bool) ([]wallet.Txn, error) {
	m.Lock()
	defer m.Unlock()
	var txns []wallet.Txn
	for txid, t := range m.txns {
		txn := wallet.Txn{
			Txid:      txid,
			Value:     int64(t.value),
			Height:    int32(t.height),
			Timestamp: t.timestamp,
			WatchOnly: t.watchOnly,
			Bytes:     t.txn,
		}
		txns = append(txns, txn)
	}
	return txns, nil
}

func (m *MockTxnStore) UpdateHeight(txid chainhash.Hash, height int, timestamp time.Time) error {
	m.Lock()
	defer m.Unlock()
	txn, ok := m.txns[txid.String()]
	if !ok {
		return errors.New("Not found")
	}
	txn.height = height
	txn.timestamp = timestamp
	m.txns[txid.String()] = txn
	return nil
}

func (m *MockTxnStore) Delete(txid *chainhash.Hash) error {
	m.Lock()
	defer m.Unlock()
	_, ok := m.txns[txid.String()]
	if !ok {
		return errors.New("Not found")
	}
	delete(m.txns, txid.String())
	return nil
}

type MockWatchedScriptsStore struct {
	scripts map[string][]byte
	sync.Mutex
}

func (m *MockWatchedScriptsStore) PutAll(scriptPubKeys [][]byte) error {
	m.Lock()
	defer m.Unlock()
	for _, scriptPubKey := range scriptPubKeys {
		m.scripts[hex.EncodeToString(scriptPubKey)] = scriptPubKey
	}
	return nil
}

func (m *MockWatchedScriptsStore) Put(scriptPubKey []byte) error {
	m.Lock()
	defer m.Unlock()
	m.scripts[hex.EncodeToString(scriptPubKey)] = scriptPubKey
	return nil
}

func (m *MockWatchedScriptsStore) GetAll() ([][]byte, error) {
	m.Lock()
	defer m.Unlock()
	var ret [][]byte
	for _, b := range m.scripts {
		ret = append(ret, b)
	}
	return ret, nil
}

func (m *MockWatchedScriptsStore) Delete(scriptPubKey []byte) error {
	m.Lock()
	defer m.Unlock()
	enc := hex.EncodeToString(scriptPubKey)
	_, ok := m.scripts[enc]
	if !ok {
		return errors.New("Not found")
	}
	delete(m.scripts, enc)
	return nil
}
