package spvwallet

import (
	"bytes"
	"encoding/hex"
	"errors"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"sort"
	"strconv"
	"testing"
	"time"
)

type MockDatastore struct {
	keys           wallet.Keys
	utxos          wallet.Utxos
	stxos          wallet.Stxos
	txns           wallet.Txns
	watchedScripts wallet.WatchedScripts
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

type keyStoreEntry struct {
	scriptAddress []byte
	path          wallet.KeyPath
	used          bool
	key           *btcec.PrivateKey
}

type mockKeyStore struct {
	keys map[string]*keyStoreEntry
}

func (m *mockKeyStore) Put(scriptAddress []byte, keyPath wallet.KeyPath) error {
	m.keys[hex.EncodeToString(scriptAddress)] = &keyStoreEntry{scriptAddress, keyPath, false, nil}
	return nil
}

func (m *mockKeyStore) ImportKey(scriptAddress []byte, key *btcec.PrivateKey) error {
	kp := wallet.KeyPath{Purpose: wallet.EXTERNAL, Index: -1}
	m.keys[hex.EncodeToString(scriptAddress)] = &keyStoreEntry{scriptAddress, kp, false, key}
	return nil
}

func (m *mockKeyStore) MarkKeyAsUsed(scriptAddress []byte) error {
	key, ok := m.keys[hex.EncodeToString(scriptAddress)]
	if !ok {
		return errors.New("key does not exist")
	}
	key.used = true
	return nil
}

func (m *mockKeyStore) GetLastKeyIndex(purpose wallet.KeyPurpose) (int, bool, error) {
	i := -1
	used := false
	for _, key := range m.keys {
		if key.path.Purpose == purpose && key.path.Index > i {
			i = key.path.Index
			used = key.used
		}
	}
	if i == -1 {
		return i, used, errors.New("No saved keys")
	}
	return i, used, nil
}

func (m *mockKeyStore) GetPathForKey(scriptAddress []byte) (wallet.KeyPath, error) {
	key, ok := m.keys[hex.EncodeToString(scriptAddress)]
	if !ok || key.path.Index == -1 {
		return wallet.KeyPath{}, errors.New("key does not exist")
	}
	return key.path, nil
}

func (m *mockKeyStore) GetKey(scriptAddress []byte) (*btcec.PrivateKey, error) {
	for _, k := range m.keys {
		if k.path.Index == -1 && bytes.Equal(scriptAddress, k.scriptAddress) {
			return k.key, nil
		}
	}
	return nil, errors.New("Not found")
}

func (m *mockKeyStore) GetImported() ([]*btcec.PrivateKey, error) {
	var keys []*btcec.PrivateKey
	for _, k := range m.keys {
		if k.path.Index == -1 {
			keys = append(keys, k.key)
		}
	}
	return keys, nil
}

func (m *mockKeyStore) GetUnused(purpose wallet.KeyPurpose) ([]int, error) {
	var i []int
	for _, key := range m.keys {
		if !key.used && key.path.Purpose == purpose {
			i = append(i, key.path.Index)
		}
	}
	sort.Ints(i)
	return i, nil
}

func (m *mockKeyStore) GetAll() ([]wallet.KeyPath, error) {
	var kp []wallet.KeyPath
	for _, key := range m.keys {
		kp = append(kp, key.path)
	}
	return kp, nil
}

func (m *mockKeyStore) GetLookaheadWindows() map[wallet.KeyPurpose]int {
	internalLastUsed := -1
	externalLastUsed := -1
	for _, key := range m.keys {
		if key.path.Purpose == wallet.INTERNAL && key.used && key.path.Index > internalLastUsed {
			internalLastUsed = key.path.Index
		}
		if key.path.Purpose == wallet.EXTERNAL && key.used && key.path.Index > externalLastUsed {
			externalLastUsed = key.path.Index
		}
	}
	internalUnused := 0
	externalUnused := 0
	for _, key := range m.keys {
		if key.path.Purpose == wallet.INTERNAL && !key.used && key.path.Index > internalLastUsed {
			internalUnused++
		}
		if key.path.Purpose == wallet.EXTERNAL && !key.used && key.path.Index > externalLastUsed {
			externalUnused++
		}
	}
	mp := make(map[wallet.KeyPurpose]int)
	mp[wallet.INTERNAL] = internalUnused
	mp[wallet.EXTERNAL] = externalUnused
	return mp
}

type mockUtxoStore struct {
	utxos map[string]*wallet.Utxo
}

func (m *mockUtxoStore) Put(utxo wallet.Utxo) error {
	key := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	m.utxos[key] = &utxo
	return nil
}

func (m *mockUtxoStore) GetAll() ([]wallet.Utxo, error) {
	var utxos []wallet.Utxo
	for _, v := range m.utxos {
		utxos = append(utxos, *v)
	}
	return utxos, nil
}

func (m *mockUtxoStore) SetWatchOnly(utxo wallet.Utxo) error {
	key := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	u, ok := m.utxos[key]
	if !ok {
		return errors.New("Not found")
	}
	u.WatchOnly = true
	return nil
}

func (m *mockUtxoStore) Delete(utxo wallet.Utxo) error {
	key := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	_, ok := m.utxos[key]
	if !ok {
		return errors.New("Not found")
	}
	delete(m.utxos, key)
	return nil
}

type mockStxoStore struct {
	stxos map[string]*wallet.Stxo
}

func (m *mockStxoStore) Put(stxo wallet.Stxo) error {
	m.stxos[stxo.SpendTxid.String()] = &stxo
	return nil
}

func (m *mockStxoStore) GetAll() ([]wallet.Stxo, error) {
	var stxos []wallet.Stxo
	for _, v := range m.stxos {
		stxos = append(stxos, *v)
	}
	return stxos, nil
}

func (m *mockStxoStore) Delete(stxo wallet.Stxo) error {
	_, ok := m.stxos[stxo.SpendTxid.String()]
	if !ok {
		return errors.New("Not found")
	}
	delete(m.stxos, stxo.SpendTxid.String())
	return nil
}

type mockTxnStore struct {
	txns map[string]*wallet.Txn
}

func (m *mockTxnStore) Put(raw []byte, txid string, value, height int, timestamp time.Time, watchOnly bool) error {
	m.txns[txid] = &wallet.Txn{
		Txid:      txid,
		Value:     int64(value),
		Height:    int32(height),
		Timestamp: timestamp,
		WatchOnly: watchOnly,
		Bytes:     raw,
	}
	return nil
}

func (m *mockTxnStore) Get(txid chainhash.Hash) (wallet.Txn, error) {
	t, ok := m.txns[txid.String()]
	if !ok {
		return wallet.Txn{}, errors.New("Not found")
	}
	return *t, nil
}

func (m *mockTxnStore) GetAll(includeWatchOnly bool) ([]wallet.Txn, error) {
	var txns []wallet.Txn
	for _, t := range m.txns {
		if !includeWatchOnly && t.WatchOnly {
			continue
		}
		txns = append(txns, *t)
	}
	return txns, nil
}

func (m *mockTxnStore) UpdateHeight(txid chainhash.Hash, height int) error {
	txn, ok := m.txns[txid.String()]
	if !ok {
		return errors.New("Not found")
	}
	txn.Height = int32(height)
	return nil
}

func (m *mockTxnStore) Delete(txid *chainhash.Hash) error {
	_, ok := m.txns[txid.String()]
	if !ok {
		return errors.New("Not found")
	}
	delete(m.txns, txid.String())
	return nil
}

type mockWatchedScriptsStore struct {
	scripts map[string][]byte
}

func (m *mockWatchedScriptsStore) Put(scriptPubKey []byte) error {
	m.scripts[hex.EncodeToString(scriptPubKey)] = scriptPubKey
	return nil
}

func (m *mockWatchedScriptsStore) GetAll() ([][]byte, error) {
	var ret [][]byte
	for _, b := range m.scripts {
		ret = append(ret, b)
	}
	return ret, nil
}

func (m *mockWatchedScriptsStore) Delete(scriptPubKey []byte) error {
	enc := hex.EncodeToString(scriptPubKey)
	_, ok := m.scripts[enc]
	if !ok {
		return errors.New("Not found")
	}
	delete(m.scripts, enc)
	return nil
}

func TestUtxo_IsEqual(t *testing.T) {
	h, err := chainhash.NewHashFromStr("16bed6368b8b1542cd6eb87f5bc20dc830b41a2258dde40438a75fa701d24e9a")
	if err != nil {
		t.Error(err)
	}
	u := &wallet.Utxo{
		Op:           *wire.NewOutPoint(h, 0),
		ScriptPubkey: make([]byte, 32),
		AtHeight:     400000,
		Value:        1000000,
	}
	if !u.IsEqual(u) {
		t.Error("Failed to return utxos as equal")
	}
	testUtxo := *u
	testUtxo.Op.Index = 3
	if u.IsEqual(&testUtxo) {
		t.Error("Failed to return utxos as not equal")
	}
	testUtxo = *u
	testUtxo.AtHeight = 1
	if u.IsEqual(&testUtxo) {
		t.Error("Failed to return utxos as not equal")
	}
	testUtxo = *u
	testUtxo.Value = 4
	if u.IsEqual(&testUtxo) {
		t.Error("Failed to return utxos as not equal")
	}
	testUtxo = *u
	ch2, err := chainhash.NewHashFromStr("1f64249abbf2fcc83fc060a64f69a91391e9f5d98c5d3135fe9716838283aa4c")
	if err != nil {
		t.Error(err)
	}
	testUtxo.Op.Hash = *ch2
	if u.IsEqual(&testUtxo) {
		t.Error("Failed to return utxos as not equal")
	}
	testUtxo = *u
	testUtxo.ScriptPubkey = make([]byte, 4)
	if u.IsEqual(&testUtxo) {
		t.Error("Failed to return utxos as not equal")
	}
	if u.IsEqual(nil) {
		t.Error("Failed to return utxos as not equal")
	}
}

func TestStxo_IsEqual(t *testing.T) {
	h, err := chainhash.NewHashFromStr("16bed6368b8b1542cd6eb87f5bc20dc830b41a2258dde40438a75fa701d24e9a")
	if err != nil {
		t.Error(err)
	}
	u := &wallet.Utxo{
		Op:           *wire.NewOutPoint(h, 0),
		ScriptPubkey: make([]byte, 32),
		AtHeight:     400000,
		Value:        1000000,
	}
	h2, err := chainhash.NewHashFromStr("1f64249abbf2fcc83fc060a64f69a91391e9f5d98c5d3135fe9716838283aa4c")
	s := &wallet.Stxo{
		Utxo:        *u,
		SpendHeight: 400001,
		SpendTxid:   *h2,
	}
	if !s.IsEqual(s) {
		t.Error("Failed to return stxos as equal")
	}

	testStxo := *s
	testStxo.SpendHeight = 5
	if s.IsEqual(&testStxo) {
		t.Error("Failed to return stxos as not equal")
	}
	h3, err := chainhash.NewHashFromStr("3c5cea030a432ba9c8cf138a93f7b2e5b28263ea416894ee0bdf91bc31bb04f2")
	testStxo = *s
	testStxo.SpendTxid = *h3
	if s.IsEqual(&testStxo) {
		t.Error("Failed to return stxos as not equal")
	}
	if s.IsEqual(nil) {
		t.Error("Failed to return stxos as not equal")
	}
	testStxo = *s
	testStxo.Utxo.AtHeight = 7
	if s.IsEqual(&testStxo) {
		t.Error("Failed to return stxos as not equal")
	}
}
