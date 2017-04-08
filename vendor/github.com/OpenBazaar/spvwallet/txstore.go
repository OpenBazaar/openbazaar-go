package spvwallet

import (
	"bytes"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/bloom"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"sync"
	"time"
)

const FlagPrefix = 0x00

type TxStore struct {
	Adrs           []btcutil.Address
	watchedScripts [][]byte
	addrMutex      *sync.Mutex
	cbMutex        *sync.Mutex

	Param *chaincfg.Params

	internalKey *hd.ExtendedKey
	externalKey *hd.ExtendedKey

	listeners []func(TransactionCallback)

	Datastore
}

func NewTxStore(p *chaincfg.Params, db Datastore, masterPrivKey *hd.ExtendedKey) (*TxStore, error) {
	// Derive keys using Bip44
	fourtyFour, err := masterPrivKey.Child(hd.HardenedKeyStart + 44)
	if err != nil {
		return nil, err
	}
	bitcoin, err := fourtyFour.Child(hd.HardenedKeyStart + 0)
	if err != nil {
		return nil, err
	}
	account, err := bitcoin.Child(hd.HardenedKeyStart + 0)
	if err != nil {
		return nil, err
	}
	external, err := account.Child(0)
	if err != nil {
		return nil, err
	}
	internal, err := account.Child(1)
	if err != nil {
		return nil, err
	}
	txs := &TxStore{
		Param:       p,
		externalKey: external,
		internalKey: internal,
		addrMutex:   new(sync.Mutex),
		cbMutex:     new(sync.Mutex),
		Datastore:   db,
	}
	err = txs.PopulateAdrs()
	if err != nil {
		return nil, err
	}
	return txs, nil
}

// ... or I'm gonna fade away
func (ts *TxStore) GimmeFilter() (*bloom.Filter, error) {
	ts.PopulateAdrs()

	// get all utxos to add outpoints to filter
	allUtxos, err := ts.Utxos().GetAll()
	if err != nil {
		return nil, err
	}

	allStxos, err := ts.Stxos().GetAll()
	if err != nil {
		return nil, err
	}
	ts.addrMutex.Lock()
	elem := uint32(len(ts.Adrs) + len(allUtxos) + len(allStxos))
	f := bloom.NewFilter(elem, 0, 0.0001, wire.BloomUpdateAll)

	// note there could be false positives since we're just looking
	// for the 20 byte PKH without the opcodes.
	for _, a := range ts.Adrs { // add 20-byte pubkeyhash
		f.Add(a.ScriptAddress())
	}
	ts.addrMutex.Unlock()
	for _, u := range allUtxos {
		f.AddOutPoint(&u.Op)
	}

	for _, s := range allStxos {
		f.AddOutPoint(&s.Utxo.Op)
	}
	for _, w := range ts.watchedScripts {
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(w, ts.Param)
		if err != nil {
			continue
		}
		f.Add(addrs[0].ScriptAddress())
	}

	return f, nil
}

// GetDoubleSpends takes a transaction and compares it with
// all transactions in the db.  It returns a slice of all txids in the db
// which are double spent by the received tx.
func (ts *TxStore) CheckDoubleSpends(argTx *wire.MsgTx) ([]*chainhash.Hash, error) {
	var dubs []*chainhash.Hash // slice of all double-spent txs
	argTxid := argTx.TxHash()
	txs, err := ts.Txns().GetAll(true)
	if err != nil {
		return dubs, err
	}
	for _, compTx := range txs {
		r := bytes.NewReader(compTx.Bytes)
		msgTx := wire.NewMsgTx(1)
		msgTx.BtcDecode(r, 1)
		compTxid := msgTx.TxHash()
		for _, argIn := range argTx.TxIn {
			// iterate through inputs of compTx
			for _, compIn := range msgTx.TxIn {
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

// GetPendingInv returns an inv message containing all txs known to the
// db which are at height 0 (not known to be confirmed).
// This can be useful on startup or to rebroadcast unconfirmed txs.
func (ts *TxStore) GetPendingInv() (*wire.MsgInv, error) {
	// use a map (really a set) do avoid dupes
	txidMap := make(map[chainhash.Hash]struct{})

	utxos, err := ts.Utxos().GetAll() // get utxos from db
	if err != nil {
		return nil, err
	}
	stxos, err := ts.Stxos().GetAll() // get stxos from db
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
	ts.watchedScripts, _ = ts.WatchedScripts().GetAll()
	ts.addrMutex.Unlock()
	return nil
}

// Ingest puts a tx into the DB atomically.  This can result in a
// gain, a loss, or no result.  Gain or loss in satoshis is returned.
func (ts *TxStore) Ingest(tx *wire.MsgTx, height int32) (uint32, error) {
	var hits uint32
	var err error
	// Tx has been OK'd by SPV; check tx sanity
	utilTx := btcutil.NewTx(tx) // convert for validation
	// Checks basic stuff like there are inputs and ouputs
	err = blockchain.CheckTransactionSanity(utilTx)
	if err != nil {
		return hits, err
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
	PKscripts := make([][]byte, len(ts.Adrs))
	for i, _ := range ts.Adrs {
		// Iterate through all our addresses
		PKscripts[i], err = txscript.PayToAddrScript(ts.Adrs[i])
		if err != nil {
			return hits, err
		}
	}
	ts.addrMutex.Unlock()
	cachedSha := tx.TxHash()
	// Iterate through all outputs of this tx, see if we gain
	cb := TransactionCallback{Txid: cachedSha.CloneBytes(), Height: height}
	value := int64(0)
	matchesWatchOnly := false
	for i, txout := range tx.TxOut {
		out := TransactionOutput{ScriptPubKey: txout.PkScript, Value: txout.Value, Index: uint32(i)}
		for _, script := range PKscripts {
			if bytes.Equal(txout.PkScript, script) { // new utxo found
				ts.Keys().MarkKeyAsUsed(txout.PkScript)
				newop := wire.OutPoint{
					Hash:  cachedSha,
					Index: uint32(i),
				}
				newu := Utxo{
					AtHeight:     height,
					Value:        txout.Value,
					ScriptPubkey: txout.PkScript,
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
			if bytes.Equal(txout.PkScript, script) {
				newop := wire.OutPoint{
					Hash:  cachedSha,
					Index: uint32(i),
				}
				newu := Utxo{
					AtHeight:     height,
					Value:        txout.Value,
					ScriptPubkey: txout.PkScript,
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
	for _, txin := range tx.TxIn {
		for i, u := range utxos {
			if outPointsEqual(txin.PreviousOutPoint, u.Op) {
				st := Stxo{
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

				in := TransactionInput{
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
				break
			}
		}
	}

	// If hits is nonzero it's a relevant tx and we should store it
	if hits > 0 || matchesWatchOnly {
		ts.cbMutex.Lock()
		_, txn, err := ts.Txns().Get(tx.TxHash())
		shouldCallback := false
		if err != nil {
			cb.Value = value
			txn.Timestamp = time.Now()
			shouldCallback = true
			ts.Txns().Put(tx, int(value), int(height), txn.Timestamp, hits == 0)
		}
		// Let's check the height before committing so we don't allow rogue peers to send us a lose
		// tx that resets our height to zero.
		if txn.Height <= 0 {
			ts.Txns().UpdateHeight(tx.TxHash(), int(height))
			if height > 0 {
				cb.Value = txn.Value
				shouldCallback = true
			}
		}
		if shouldCallback {
			// Callback on listeners
			for _, listener := range ts.listeners {
				listener(cb)
			}
		}
		ts.cbMutex.Unlock()
		ts.PopulateAdrs()
	}
	return hits, err
}

func (ts *TxStore) markAsDead(txid chainhash.Hash) error {
	stxos, err := ts.Stxos().GetAll()
	if err != nil {
		return err
	}
	markStxoAsDead := func(s Stxo) error {
		err := ts.Stxos().Delete(s)
		if err != nil {
			return err
		}
		err = ts.Txns().UpdateHeight(txid, -1)
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
		// If an dependency of the spend is dead then mark the spend as dead
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
			ts.Txns().UpdateHeight(txid, -1)
		}
	}
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

func outPointsEqual(a, b wire.OutPoint) bool {
	if !a.Hash.IsEqual(&b.Hash) {
		return false
	}
	return a.Index == b.Index
}
