package zcash

import (
	"bytes"
	"sync"
	"time"

	"github.com/OpenBazaar/multiwallet/keys"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

type TxStore struct {
	adrs           []btcutil.Address
	watchedScripts [][]byte
	txids          map[string]int32
	addrMutex      *sync.Mutex
	cbMutex        *sync.Mutex

	keyManager *keys.KeyManager

	params *chaincfg.Params

	listeners []func(wallet.TransactionCallback)

	wallet.Datastore
}

func NewTxStore(p *chaincfg.Params, db wallet.Datastore, keyManager *keys.KeyManager) (*TxStore, error) {
	txs := &TxStore{
		params:     p,
		keyManager: keyManager,
		addrMutex:  new(sync.Mutex),
		cbMutex:    new(sync.Mutex),
		txids:      make(map[string]int32),
		Datastore:  db,
	}
	err := txs.PopulateAdrs()
	if err != nil {
		return nil, err
	}
	return txs, nil
}

// GetDoubleSpends takes a transaction and compares it with
// all transactions in the db.  It returns a slice of all txids in the db
// which are double spent by the received tx.
func (ts *TxStore) CheckDoubleSpends(argTx *Transaction) ([]*chainhash.Hash, error) {
	var dubs []*chainhash.Hash // slice of all double-spent txs
	argTxid := argTx.TxHash()
	txs, err := ts.Txns().GetAll(true)
	if err != nil {
		return dubs, err
	}
	for _, compTx := range txs {
		if compTx.Height < 0 {
			continue
		}
		r := bytes.NewReader(compTx.Bytes)
		var msgTx Transaction
		if _, err := msgTx.ReadFrom(r); err != nil {
			return nil, err
		}
		compTxid := msgTx.TxHash()
		for _, argIn := range argTx.Inputs {
			// iterate through inputs of compTx
			for _, compIn := range msgTx.Inputs {
				if outPointsEqual(argIn.PreviousOutPoint, compIn.PreviousOutPoint) && !compTxid.IsEqual(&argTxid) {
					// found double spend
					dubs = append(dubs, &compTxid)
					break // back to argIn loop
				}
			}
		}
	}
	return dubs, nil
}

// PopulateAdrs just puts a bunch of adrs in ram; it doesn't touch the DB
func (ts *TxStore) PopulateAdrs() error {
	keys := ts.keyManager.GetKeys()
	ts.addrMutex.Lock()
	ts.adrs = []btcutil.Address{}
	for _, k := range keys {
		addr, err := KeyToAddress(k, ts.params)
		if err != nil {
			continue
		}
		ts.adrs = append(ts.adrs, addr)
	}
	ts.watchedScripts, _ = ts.WatchedScripts().GetAll()
	txns, _ := ts.Txns().GetAll(true)
	for _, t := range txns {
		ts.txids[t.Txid] = t.Height
	}
	ts.addrMutex.Unlock()
	return nil
}

// Ingest puts a tx into the DB atomically.  This can result in a
// gain, a loss, or no result.  Gain or loss in satoshis is returned.
func (ts *TxStore) Ingest(tx *Transaction, raw []byte, height int32, timestamp time.Time) (uint32, error) {
	var hits uint32
	var err error
	if err := tx.Validate(ts.params); err != nil {
		return hits, err
	}

	// Check to see if we've already processed this tx. If so, return.
	sh, ok := ts.txids[tx.TxHash().String()]
	if ok && (sh > 0 || (sh == 0 && height == 0)) {
		return 1, nil
	}

	// Check to see if this is a double spend
	doubleSpends, err := ts.CheckDoubleSpends(tx)
	if err != nil {
		return hits, err
	}
	if len(doubleSpends) > 0 {
		// First seen rule
		if height == 0 {
			return 0, nil
		} else {
			// Mark any unconfirmed doubles as dead
			for _, double := range doubleSpends {
				ts.markAsDead(*double)
			}
		}
	}

	// Generate PKscripts for all addresses
	ts.addrMutex.Lock()
	PKscripts := make([][]byte, len(ts.adrs))
	for i := range ts.adrs {
		// Iterate through all our addresses
		// TODO: This will need to test both segwit and legacy once segwit activates
		PKscripts[i], err = PayToAddrScript(ts.adrs[i])
		if err != nil {
			return hits, err
		}
	}
	ts.addrMutex.Unlock()

	// Iterate through all outputs of this tx, see if we gain
	cachedSha := tx.TxHash()
	cb := wallet.TransactionCallback{Txid: cachedSha.String(), Height: height}
	value := int64(0)
	matchesWatchOnly := false
	for i, txout := range tx.Outputs {
		out := wallet.TransactionOutput{ScriptPubKey: txout.ScriptPubKey, Value: txout.Value, Index: uint32(i)}
		for _, script := range PKscripts {
			if bytes.Equal(txout.ScriptPubKey, script) { // new utxo found
				scriptAddress, _ := ts.extractScriptAddress(txout.ScriptPubKey)
				ts.keyManager.MarkKeyAsUsed(scriptAddress)
				newop := wire.OutPoint{
					Hash:  cachedSha,
					Index: uint32(i),
				}
				newu := wallet.Utxo{
					AtHeight:     height,
					Value:        txout.Value,
					ScriptPubkey: txout.ScriptPubKey,
					Op:           newop,
					WatchOnly:    false,
				}
				value += newu.Value
				ts.Utxos().Put(newu)
				hits++
				break
			}
		}
		// Now check watched scripts
		for _, script := range ts.watchedScripts {
			if bytes.Equal(txout.ScriptPubKey, script) {
				newop := wire.OutPoint{
					Hash:  cachedSha,
					Index: uint32(i),
				}
				newu := wallet.Utxo{
					AtHeight:     height,
					Value:        txout.Value,
					ScriptPubkey: txout.ScriptPubKey,
					Op:           newop,
					WatchOnly:    true,
				}
				ts.Utxos().Put(newu)
				matchesWatchOnly = true
			}
		}
		cb.Outputs = append(cb.Outputs, out)
	}
	utxos, err := ts.Utxos().GetAll()
	if err != nil {
		return 0, err
	}
	for _, txin := range tx.Inputs {
		for i, u := range utxos {
			if outPointsEqual(txin.PreviousOutPoint, u.Op) {
				st := wallet.Stxo{
					Utxo:        u,
					SpendHeight: height,
					SpendTxid:   cachedSha,
				}
				ts.Stxos().Put(st)
				ts.Utxos().Delete(u)
				utxos = append(utxos[:i], utxos[i+1:]...)
				if !u.WatchOnly {
					value -= u.Value
					hits++
				} else {
					matchesWatchOnly = true
				}

				in := wallet.TransactionInput{
					OutpointHash:       u.Op.Hash.CloneBytes(),
					OutpointIndex:      u.Op.Index,
					LinkedScriptPubKey: u.ScriptPubkey,
					Value:              u.Value,
				}
				cb.Inputs = append(cb.Inputs, in)
				break
			}
		}
	}

	// Update height of any stxos
	if height > 0 {
		stxos, err := ts.Stxos().GetAll()
		if err != nil {
			return 0, err
		}
		for _, stxo := range stxos {
			if stxo.SpendTxid.IsEqual(&cachedSha) {
				stxo.SpendHeight = height
				ts.Stxos().Put(stxo)
				if !stxo.Utxo.WatchOnly {
					hits++
				} else {
					matchesWatchOnly = true
				}
				break
			}
		}
	}

	// If hits is nonzero it's a relevant tx and we should store it
	if hits > 0 || matchesWatchOnly {
		ts.cbMutex.Lock()
		txn, err := ts.Txns().Get(tx.TxHash())
		shouldCallback := false
		if err != nil {
			cb.Value = value
			txn.Timestamp = timestamp
			shouldCallback = true
			ts.Txns().Put(raw, tx.TxHash().String(), int(value), int(height), txn.Timestamp, hits == 0)
			ts.txids[tx.TxHash().String()] = height
		}
		// Let's check the height before committing so we don't allow rogue peers to send us a lose
		// tx that resets our height to zero.
		if err == nil && txn.Height <= 0 {
			ts.Txns().UpdateHeight(tx.TxHash(), int(height), txn.Timestamp)
			ts.txids[tx.TxHash().String()] = height
			if height > 0 {
				cb.Value = txn.Value
				shouldCallback = true
			}
		}
		cb.BlockTime = timestamp
		if shouldCallback {
			// Callback on listeners
			for _, listener := range ts.listeners {
				listener(cb)
			}
		}
		ts.cbMutex.Unlock()
		ts.PopulateAdrs()
		hits++
	}
	return hits, err
}

func (ts *TxStore) markAsDead(txid chainhash.Hash) error {
	stxos, err := ts.Stxos().GetAll()
	if err != nil {
		return err
	}
	markStxoAsDead := func(s wallet.Stxo) error {
		err := ts.Stxos().Delete(s)
		if err != nil {
			return err
		}
		err = ts.Txns().UpdateHeight(s.SpendTxid, -1, time.Now())
		if err != nil {
			return err
		}
		return nil
	}
	for _, s := range stxos {
		// If an stxo is marked dead, move it back into the utxo table
		if txid.IsEqual(&s.SpendTxid) {
			if err := markStxoAsDead(s); err != nil {
				return err
			}
			if err := ts.Utxos().Put(s.Utxo); err != nil {
				return err
			}
		}
		// If a dependency of the spend is dead then mark the spend as dead
		if txid.IsEqual(&s.Utxo.Op.Hash) {
			if err := markStxoAsDead(s); err != nil {
				return err
			}
			if err := ts.markAsDead(s.SpendTxid); err != nil {
				return err
			}
		}
	}
	utxos, err := ts.Utxos().GetAll()
	if err != nil {
		return err
	}
	// Dead utxos should just be deleted
	for _, u := range utxos {
		if txid.IsEqual(&u.Op.Hash) {
			err := ts.Utxos().Delete(u)
			if err != nil {
				return err
			}
		}
	}
	ts.Txns().UpdateHeight(txid, -1, time.Now())
	return nil
}

func (ts *TxStore) processReorg(lastGoodHeight uint32) error {
	txns, err := ts.Txns().GetAll(true)
	if err != nil {
		return err
	}
	for i := len(txns) - 1; i >= 0; i-- {
		if txns[i].Height > int32(lastGoodHeight) {
			txid, err := chainhash.NewHashFromStr(txns[i].Txid)
			if err != nil {
				log.Error(err)
				continue
			}
			err = ts.markAsDead(*txid)
			if err != nil {
				log.Error(err)
				continue
			}
		}
	}
	return nil
}

func (ts *TxStore) extractScriptAddress(script []byte) ([]byte, error) {
	addr, err := ExtractPkScriptAddrs(script, ts.params)
	if err != nil {
		return nil, err
	}
	return addr.ScriptAddress(), nil
}

func outPointsEqual(a, b wire.OutPoint) bool {
	if !a.Hash.IsEqual(&b.Hash) {
		return false
	}
	return a.Index == b.Index
}
