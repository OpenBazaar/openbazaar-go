package libbitcoin

import (
	"encoding/hex"
	btc "github.com/btcsuite/btcutil"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"bytes"
	"time"
)

// Process a transaction that comes off the wire. A transaction could be passed in here for three reasons:
// 1. It's a new transaction fresh off the wire.
// 2. It's our own outgoing transaction.
// 3. It's a transaction whose state has updated (ie. recently confirmed)
// In cases 1 and 2 we add the transaction to the database and update our utxo table.
// In the last case we just update the height and/or state of the transaction.
func (w *LibbitcoinWallet) ProcessTransaction(tx *btc.Tx, height uint32) {
	txid, err := hex.DecodeString(tx.Sha().String())
	if err != nil {
		return
	}
	value := 0
	if !w.db.Transactions().Has(txid) {
		// If output sends coins to one of our scripts, save it in the utxo db and mark the key as used.
		for i, output := range(tx.MsgTx().TxOut) {
			key, err := w.db.Keys().GetKeyForScript(output.PkScript)
			if err == nil {
				w.db.Coins().Put(bitcoin.Utxo{
					Txid: txid,
					Index: i,
					Value: int(output.Value),
					ScriptPubKey: output.PkScript,
				})
				w.db.Keys().MarkKeyAsUsed(key)
				value += int(output.Value)
			}
		}
		// If input exists in utxo db, delete it
		for _, input := range(tx.MsgTx().TxIn) {
			outpointTxid, err := hex.DecodeString(input.PreviousOutPoint.Hash.String())
			if err != nil {
				return
			}
			if w.db.Coins().Has(outpointTxid, int(input.PreviousOutPoint.Index)) {
				v, err := w.db.Coins().GetValue(outpointTxid, int(input.PreviousOutPoint.Index))
				if err != nil {
					return
				}
				value -= v
				w.db.Coins().Delete(outpointTxid, int(input.PreviousOutPoint.Index))
			}
		}

		// Put to database
		serializedTx := new(bytes.Buffer)
		tx.MsgTx().Serialize(serializedTx)
		var state bitcoin.TransactionState
		if height > 0 {
			state = bitcoin.CONFIRMED
		} else {
			state = bitcoin.PENDING
		}
		w.db.Transactions().Put(bitcoin.TransactionInfo{
			Txid: txid,
			Tx: serializedTx.Bytes(),
			Height: int(height),
			State: state,
			Timestamp: time.Now(),
			Value: value,
			ExchangeRate: float64(0),
			ExchangCurrency: "",
		})
	} else {
		if height > 0 {
			w.db.Transactions().UpdateState(txid, bitcoin.CONFIRMED)
		}
		w.db.Transactions().UpdateHeight(txid, int(height))
	}
}