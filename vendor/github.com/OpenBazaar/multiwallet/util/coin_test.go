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

func TestGatherCoins(t *testing.T) {
	scriptPubkey1 := "76a914ab8c06d1c22f575b30c3afc66bde8b3aa2de99bc88ac"
	scriptBytes1, err := hex.DecodeString(scriptPubkey1)
	if err != nil {
		t.Error(err)
		return
	}
	scriptPubkey2 := "76a914281032bc033f41a33ded636bc2f7c2d67bb2871f88ac"
	scriptBytes2, err := hex.DecodeString(scriptPubkey2)
	if err != nil {
		t.Error(err)
		return
	}
	scriptPubkey3 := "76a91450033f99ce3ed61dc428a0ac481e9bdab646664c88ac"
	scriptBytes3, err := hex.DecodeString(scriptPubkey3)
	if err != nil {
		t.Error(err)
		return
	}
	ch1, err := chainhash.NewHashFromStr("8cf466484a741850b63482133b6f7d506297c624290db2bb74214e4f9932f93e")
	if err != nil {
		t.Error(err)
		return
	}
	op1 := wire.NewOutPoint(ch1, 0)
	ch2, err := chainhash.NewHashFromStr("8fc073d5452cc2765a24baf5d434fedc1d16b7f74f9dabce209a6b416d4fb91f")
	if err != nil {
		t.Error(err)
		return
	}
	op2 := wire.NewOutPoint(ch2, 1)
	ch3, err := chainhash.NewHashFromStr("d7144e933f4a03ff194e373331d5a4ef8c5e4ce8df666c66b882145e686834b1")
	if err != nil {
		t.Error(err)
		return
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
		t.Error(err)
		return
	}
	key0, err := master.Child(0)
	if err != nil {
		t.Error(err)
		return
	}
	key1, err := master.Child(1)
	if err != nil {
		t.Error(err)
		return
	}
	key2, err := master.Child(2)
	if err != nil {
		t.Error(err)
		return
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
			return nil, errors.New("Key not found")
		}
		return key, nil
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
