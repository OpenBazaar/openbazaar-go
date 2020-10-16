package util

import (
	"bytes"
	"encoding/hex"
	"errors"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"testing"
)

func TestNewCoin(t *testing.T) {
	txid := "7eae21cc2709a58a8795f9b0239b6b8ed974a3c4ce10f8919deae527995dd744"
	ch, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		t.Error(err)
		return
	}
	scriptPubkey := "76a914ab8c06d1c22f575b30c3afc66bde8b3aa2de99bc88ac"
	scriptBytes, err := hex.DecodeString(scriptPubkey)
	if err != nil {
		t.Error(err)
		return
	}
	c, err := NewCoin(*ch, 0, btcutil.Amount(100000), 10, scriptBytes)
	if err != nil {
		t.Error(err)
		return
	}
	if c.Hash().String() != ch.String() {
		t.Error("Returned incorrect txid")
	}
	if c.Index() != 0 {
		t.Error("Returned incorrect index")
	}
	if c.Value() != btcutil.Amount(100000) {
		t.Error("Returned incorrect value")
	}
	if !bytes.Equal(c.PkScript(), scriptBytes) {
		t.Error("Returned incorrect pk script")
	}
	if c.NumConfs() != 10 {
		t.Error("Returned incorrect num confs")
	}
	if c.ValueAge() != 1000000 {
		t.Error("Returned incorrect value age")
	}
}

func buildTestData() (uint32, []wallet.Utxo, func(script []byte) (btcutil.Address, error),
	func(scriptAddress []byte) (*hd.ExtendedKey, error),
	map[string]wallet.Utxo, map[string]*hd.ExtendedKey, error) {

	scriptPubkey1 := "76a914ab8c06d1c22f575b30c3afc66bde8b3aa2de99bc88ac"
	scriptBytes1, err := hex.DecodeString(scriptPubkey1)
	if err != nil {
		return 0, nil, nil, nil, nil, nil, err
	}
	scriptPubkey2 := "76a914281032bc033f41a33ded636bc2f7c2d67bb2871f88ac"
	scriptBytes2, err := hex.DecodeString(scriptPubkey2)
	if err != nil {
		return 0, nil, nil, nil, nil, nil, err
	}
	scriptPubkey3 := "76a91450033f99ce3ed61dc428a0ac481e9bdab646664c88ac"
	scriptBytes3, err := hex.DecodeString(scriptPubkey3)
	if err != nil {
		return 0, nil, nil, nil, nil, nil, err
	}
	ch1, err := chainhash.NewHashFromStr("8cf466484a741850b63482133b6f7d506297c624290db2bb74214e4f9932f93e")
	if err != nil {
		return 0, nil, nil, nil, nil, nil, err
	}
	op1 := wire.NewOutPoint(ch1, 0)
	ch2, err := chainhash.NewHashFromStr("8fc073d5452cc2765a24baf5d434fedc1d16b7f74f9dabce209a6b416d4fb91f")
	if err != nil {
		return 0, nil, nil, nil, nil, nil, err
	}
	op2 := wire.NewOutPoint(ch2, 1)
	ch3, err := chainhash.NewHashFromStr("d7144e933f4a03ff194e373331d5a4ef8c5e4ce8df666c66b882145e686834b1")
	if err != nil {
		return 0, nil, nil, nil, nil, nil, err
	}
	op3 := wire.NewOutPoint(ch3, 2)
	utxos := []wallet.Utxo{
		{
			Value:        100000,
			WatchOnly:    false,
			AtHeight:     300000,
			ScriptPubkey: scriptBytes1,
			Op:           *op1,
		},
		{
			Value:        50000,
			WatchOnly:    false,
			AtHeight:     350000,
			ScriptPubkey: scriptBytes2,
			Op:           *op2,
		},
		{
			Value:        99000,
			WatchOnly:    true,
			AtHeight:     250000,
			ScriptPubkey: scriptBytes3,
			Op:           *op3,
		},
	}

	utxoMap := make(map[string]wallet.Utxo)
	utxoMap[utxos[0].Op.Hash.String()] = utxos[0]
	utxoMap[utxos[1].Op.Hash.String()] = utxos[1]
	utxoMap[utxos[2].Op.Hash.String()] = utxos[2]

	master, err := hd.NewMaster([]byte("8cf466484a741850b63482133b6f7d506297c624290db2bb74214e4f9932f93e"), &chaincfg.MainNetParams)
	if err != nil {
		return 0, nil, nil, nil, nil, nil, err
	}
	key0, err := master.Child(0)
	if err != nil {
		return 0, nil, nil, nil, nil, nil, err
	}
	key1, err := master.Child(1)
	if err != nil {
		return 0, nil, nil, nil, nil, nil, err
	}
	key2, err := master.Child(2)
	if err != nil {
		return 0, nil, nil, nil, nil, nil, err
	}

	keyMap := make(map[string]*hd.ExtendedKey)
	keyMap["ab8c06d1c22f575b30c3afc66bde8b3aa2de99bc"] = key0
	keyMap["281032bc033f41a33ded636bc2f7c2d67bb2871f"] = key1
	keyMap["50033f99ce3ed61dc428a0ac481e9bdab646664c"] = key2

	height := uint32(351000)

	scriptToAddress := func(script []byte) (btcutil.Address, error) {
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(script, &chaincfg.MainNetParams)
		if err != nil {
			return nil, err
		}
		return addrs[0], nil
	}
	getKeyForScript := func(scriptAddress []byte) (*hd.ExtendedKey, error) {
		key, ok := keyMap[hex.EncodeToString(scriptAddress)]
		if !ok {
			return nil, errors.New("key not found")
		}
		return key, nil
	}
	return height, utxos, scriptToAddress, getKeyForScript, utxoMap, keyMap, nil
}

func TestGatherCoins(t *testing.T) {

	height, utxos, scriptToAddress, getKeyForScript, utxoMap, keyMap, err := buildTestData()
	if err != nil {
		t.Fatal(err)
	}

	coins := GatherCoins(height, utxos, scriptToAddress, getKeyForScript)
	if len(coins) != 2 {
		t.Error("Returned incorrect number of coins")
	}
	for coin, key := range coins {
		u := utxoMap[coin.Hash().String()]
		addr, err := scriptToAddress(coin.PkScript())
		if err != nil {
			t.Error(err)
		}
		k := keyMap[hex.EncodeToString(addr.ScriptAddress())]
		if coin.Value() != btcutil.Amount(u.Value) {
			t.Error("Returned incorrect value")
		}
		if coin.Hash().String() != u.Op.Hash.String() {
			t.Error("Returned incorrect outpoint hash")
		}
		if coin.Index() != u.Op.Index {
			t.Error("Returned incorrect outpoint index")
		}
		if !bytes.Equal(coin.PkScript(), u.ScriptPubkey) {
			t.Error("Returned incorrect script pubkey")
		}
		if key.String() != k.String() {
			t.Error("Returned incorrect key")
		}
	}
}

func TestLoadAllInputs(t *testing.T) {
	height, utxos, scriptToAddress, getKeyForScript, _, keyMap, err := buildTestData()
	if err != nil {
		t.Fatal(err)
	}
	coins := GatherCoins(height, utxos, scriptToAddress, getKeyForScript)

	tx := wire.NewMsgTx(1)
	totalIn, inputValMap, additionalPrevScripts, additionalKeysByAddress := LoadAllInputs(tx, coins, &chaincfg.MainNetParams)

	if totalIn != 150000 {
		t.Errorf("Failed to return correct total input value: expected 150000 got %d", totalIn)
	}

	for _, u := range utxos {
		val, ok := inputValMap[u.Op]
		if !u.WatchOnly && !ok {
			t.Errorf("Missing outpoint %s in input value map", u.Op)
		}
		if u.WatchOnly && ok {
			t.Error("Watch only output found in input values map")
		}

		if !u.WatchOnly && val != u.Value {
			t.Errorf("Returned incorrect input value for outpoint %s. Expected %d, got %d", u.Op, u.Value, val)
		}

		prevScript, ok := additionalPrevScripts[u.Op]
		if !u.WatchOnly && !ok {
			t.Errorf("Missing outpoint %s in additionalPrevScripts map", u.Op)
		}
		if u.WatchOnly && ok {
			t.Error("Watch only output found in additionalPrevScripts map")
		}

		if !u.WatchOnly && !bytes.Equal(prevScript, u.ScriptPubkey) {
			t.Errorf("Returned incorrect script for script %s. Expected %x, got %x", u.Op, u.ScriptPubkey, prevScript)
		}
	}

	for _, key := range additionalKeysByAddress {
		found := false
		for _, k := range keyMap {
			priv, err := k.ECPrivKey()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(key.PrivKey.Serialize(), priv.Serialize()) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Key %s not in additionalKeysByAddress map", key.String())
		}
	}
}
