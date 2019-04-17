package util

import (
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/coinset"
	hd "github.com/btcsuite/btcutil/hdkeychain"
)

type Coin struct {
	TxHash       *chainhash.Hash
	TxIndex      uint32
	TxValue      btcutil.Amount
	TxNumConfs   int64
	ScriptPubKey []byte
}

func (c *Coin) Hash() *chainhash.Hash { return c.TxHash }
func (c *Coin) Index() uint32         { return c.TxIndex }
func (c *Coin) Value() btcutil.Amount { return c.TxValue }
func (c *Coin) PkScript() []byte      { return c.ScriptPubKey }
func (c *Coin) NumConfs() int64       { return c.TxNumConfs }
func (c *Coin) ValueAge() int64       { return int64(c.TxValue) * c.TxNumConfs }

func NewCoin(txid chainhash.Hash, index uint32, value btcutil.Amount, numConfs int64, scriptPubKey []byte) (coinset.Coin, error) {
	c := &Coin{
		TxHash:       &txid,
		TxIndex:      index,
		TxValue:      value,
		TxNumConfs:   numConfs,
		ScriptPubKey: scriptPubKey,
	}
	return coinset.Coin(c), nil
}

func GatherCoins(height uint32, utxos []wallet.Utxo, scriptToAddress func(script []byte) (btcutil.Address, error), getKeyForScript func(scriptAddress []byte) (*hd.ExtendedKey, error)) map[coinset.Coin]*hd.ExtendedKey {
	m := make(map[coinset.Coin]*hd.ExtendedKey)
	for _, u := range utxos {
		if u.WatchOnly {
			continue
		}
		var confirmations int32
		if u.AtHeight > 0 {
			confirmations = int32(height) - u.AtHeight
		}
		c, err := NewCoin(u.Op.Hash, u.Op.Index, btcutil.Amount(u.Value), int64(confirmations), u.ScriptPubkey)
		if err != nil {
			continue
		}

		addr, err := scriptToAddress(u.ScriptPubkey)
		if err != nil {
			continue
		}
		key, err := getKeyForScript(addr.ScriptAddress())
		if err != nil {
			continue
		}
		m[c] = key
	}
	return m
}

func LoadAllInputs(tx *wire.MsgTx, coinMap map[coinset.Coin]*hd.ExtendedKey, params *chaincfg.Params) (int64, map[wire.OutPoint]int64, map[wire.OutPoint][]byte, map[string]*btcutil.WIF) {
	inVals := make(map[wire.OutPoint]int64)
	totalIn := int64(0)
	additionalPrevScripts := make(map[wire.OutPoint][]byte)
	additionalKeysByAddress := make(map[string]*btcutil.WIF)

	for coin, key := range coinMap {
		outpoint := wire.NewOutPoint(coin.Hash(), coin.Index())
		in := wire.NewTxIn(outpoint, nil, nil)
		additionalPrevScripts[*outpoint] = coin.PkScript()
		tx.TxIn = append(tx.TxIn, in)
		val := int64(coin.Value().ToUnit(btcutil.AmountSatoshi))
		totalIn += val
		inVals[*outpoint] = val

		addr, err := key.Address(params)
		if err != nil {
			continue
		}
		privKey, err := key.ECPrivKey()
		if err != nil {
			continue
		}
		wif, _ := btcutil.NewWIF(privKey, params, true)
		additionalKeysByAddress[addr.EncodeAddress()] = wif
	}
	return totalIn, inVals, additionalPrevScripts, additionalKeysByAddress
}
