package spvwallet

import (
	"bytes"
	"strconv"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)


// SetDBSyncHeight sets sync height of the db, indicated the latest block
// of which it has ingested all the transactions.
func (ts *TxStore) SetDBSyncHeight(n int32) error {
	return ts.db.State().Put("TipHeight", strconv.Itoa(int(n)))
}

// SyncHeight returns the chain height to which the db has synced
func (ts *TxStore) GetDBSyncHeight() (int32, error) {
	var n int32
	h, err := ts.db.State().Get("TipHeight")
	if err != nil {
		return n, nil
	}
	height, err := strconv.Atoi(h)
	if err != nil {
		return n, nil
	}
	return int32(height), nil
}

// GetPendingInv returns an inv message containing all txs known to the
// db which are at height 0 (not known to be confirmed).
// This can be useful on startup or to rebroadcast unconfirmed txs.
func (ts *TxStore) GetPendingInv() (*wire.MsgInv, error) {
	// use a map (really a set) do avoid dupes
	txidMap := make(map[wire.ShaHash]struct{})

	utxos, err := ts.db.Utxos().GetAll() // get utxos from db
	if err != nil {
		return nil, err
	}
	stxos, err := ts.db.Stxos().GetAll() // get stxos from db
	if err != nil {
		return nil, err
	}

	// iterate through utxos, adding txids of anything with height 0
	for _, utxo := range utxos {
		if utxo.AtHeight == 0 {
			txidMap[utxo.Op.Hash] = struct{}{} // adds to map
		}
	}
	// do the same with stxos based on height at which spent
	for _, stxo := range stxos {
		if stxo.SpendHeight == 0 {
			txidMap[stxo.SpendTxid] = struct{}{}
		}
	}

	invMsg := wire.NewMsgInv()
	for txid := range txidMap {
		item := wire.NewInvVect(wire.InvTypeTx, &txid)
		err = invMsg.AddInvVect(item)
		if err != nil {
			if err != nil {
				return nil, err
			}
		}
	}

	// return inv message with all txids (maybe none)
	return invMsg, nil
}

// PopulateAdrs just puts a bunch of adrs in ram; it doesn't touch the DB
func (ts *TxStore) PopulateAdrs() error {
	ts.lookahead()
	keys := ts.GetKeys()
	ts.addrMutex.Lock()
	ts.Adrs = []btcutil.Address{}
	for _, k := range keys {
		addr, err := k.Address(ts.Param)
		if err != nil {
			continue
		}
		ts.Adrs = append(ts.Adrs, addr)
	}
	ts.addrMutex.Unlock()
	return nil
}

// Ingest puts a tx into the DB atomically.  This can result in a
// gain, a loss, or no result.  Gain or loss in satoshis is returned.
func (ts *TxStore) Ingest(tx *wire.MsgTx, height int32) (uint32, error) {
	var hits uint32
	var err error
	// tx has been OK'd by SPV; check tx sanity
	utilTx := btcutil.NewTx(tx) // convert for validation
	// checks basic stuff like there are inputs and ouputs
	err = blockchain.CheckTransactionSanity(utilTx)
	if err != nil {
		return hits, err
	}
	// note that you can't check signatures; this is SPV.
	// 0 conf SPV means pretty much nothing.  Anyone can say anything.


	// go through txouts, and then go through addresses to match

	// generate PKscripts for all addresses
	ts.addrMutex.Lock()
	PKscripts := make([][]byte, len(ts.Adrs))
	for i, _ := range ts.Adrs {
		// iterate through all our addresses
		PKscripts[i], err = txscript.PayToAddrScript(ts.Adrs[i])
		if err != nil {
			return hits, err
		}
	}
	ts.addrMutex.Unlock()
	cachedSha := tx.TxSha()
	// iterate through all outputs of this tx, see if we gain
	for i, out := range tx.TxOut {
		for _, script := range PKscripts {
			if bytes.Equal(out.PkScript, script) { // new utxo found
				ts.db.Keys().MarkKeyAsUsed(out.PkScript)
				var newu Utxo // create new utxo
				newu.AtHeight = height
				newu.Value = out.Value
				newu.ScriptPubkey = out.PkScript
				var newop wire.OutPoint
				newop.Hash = cachedSha
				newop.Index = uint32(i)
				newu.Op = newop
				ts.db.Utxos().Put(newu)
				hits++
				break // txos can match only 1 script
			}
		}
	}

	for _, txin := range tx.TxIn {
		utxos, err := ts.db.Utxos().GetAll()
		if err != nil {
			return 0, err
		}
		for _, u := range utxos {
			if OutPointsEqual(txin.PreviousOutPoint, u.Op) {
				hits++
				var st Stxo               // generate spent txo
				st.Utxo = u         // assign outpoint
				st.SpendHeight = height   // spent at height
				st.SpendTxid = cachedSha  // spent by txid
				ts.db.Stxos().Put(st)
				ts.db.Utxos().Delete(u)
			}
		}
	}

	// if hits is nonzero it's a relevant tx and we should store it
	if hits > 0 {
		ts.PopulateAdrs()
		ts.db.Txns().Put(tx)
	}
	return hits, err
}
