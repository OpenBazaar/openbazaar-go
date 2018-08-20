package util

import (
	"bytes"
	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/wire"
)

func CalcBalance(utxos []wi.Utxo, txns []wi.Txn) (confirmed, unconfirmed int64) {
	var txmap = make(map[string]wi.Txn)
	for _, tx := range txns {
		txmap[tx.Txid] = tx
	}

	for _, utxo := range utxos {
		if !utxo.WatchOnly {
			if utxo.AtHeight > 0 {
				confirmed += utxo.Value
			} else {
				if checkIfStxoIsConfirmed(utxo.Op.Hash.String(), txmap) {
					confirmed += utxo.Value
				} else {
					unconfirmed += utxo.Value
				}
			}
		}
	}
	return confirmed, unconfirmed
}

func checkIfStxoIsConfirmed(txid string, txmap map[string]wi.Txn) bool {
	// First look up tx and derserialize
	txn, ok := txmap[txid]
	if !ok {
		return false
	}
	tx := wire.NewMsgTx(1)
	rbuf := bytes.NewReader(txn.Bytes)
	err := tx.BtcDecode(rbuf, wire.ProtocolVersion, wire.WitnessEncoding)
	if err != nil {
		return false
	}

	// For each input, recursively check if confirmed
	inputsConfirmed := true
	for _, in := range tx.TxIn {
		checkTx, ok := txmap[in.PreviousOutPoint.Hash.String()]
		if ok { // Is an stxo. If confirmed we can return true. If no, we need to check the dependency.
			if checkTx.Height == 0 {
				if !checkIfStxoIsConfirmed(in.PreviousOutPoint.Hash.String(), txmap) {
					inputsConfirmed = false
				}
			}
		} else { // We don't have the tx in our db so it can't be an stxo. Return false.
			return false
		}
	}
	return inputsConfirmed
}
