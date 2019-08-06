package util

import (
	"bytes"
	"log"

	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	btc "github.com/btcsuite/btcutil"
)

// GetTxnOutputs extracts TxnOutputs from raw utxo txn bytes
func GetTxnOutputs(data []byte, params *chaincfg.Params) ([]wi.TransactionOutput, error) {
	txn := []wi.TransactionOutput{}
	tx := wire.NewMsgTx(wire.TxVersion)
	rbuf := bytes.NewReader(data)
	err := tx.BtcDecode(rbuf, wire.ProtocolVersion, wire.WitnessEncoding)
	if err != nil {
		return txn, err
	}
	for i, out := range tx.TxOut {
		var addr btc.Address
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(out.PkScript, params)
		if err != nil {
			log.Printf("error extracting address from txn pkscript: %v\n", err)
		}
		if len(addrs) == 0 {
			addr = nil
		} else {
			addr = addrs[0]
		}
		tout := wi.TransactionOutput{
			Address: addr,
			Value:   out.Value,
			Index:   uint32(i),
		}
		txn = append(txn, tout)
	}
	return txn, nil
}
