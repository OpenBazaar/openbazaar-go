package spvwallet

import (
	"fmt"
	"sync"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/bloom"
	hd "github.com/btcsuite/btcutil/hdkeychain"
)

type Datastore interface {
	Utxos() Utxos
	Stxos() Stxos
	Txns()  Txns
	Keys()  Keys
	State() State
}

type Utxos interface {
	// Put a utxo to the database
	Put(utxo Utxo) error

	// Fetch all utxos from the db
	GetAll() ([]Utxo, error)

	// Delete a utxo from the db
	Delete(utxo Utxo) error
}

type Stxos interface {
	// Put a stxo to the database
	Put(stxo Stxo) error

	// Fetch all stxos from the db
	GetAll() ([]Stxo, error)

	// Delete a stxo from the db
	Delete(stxo Stxo) error
}

type Txns interface {

	// Put a new transaction to the database
	Put(txn *wire.MsgTx) error

	// Fetch a tx given it's hash
	Get(txid wire.ShaHash) (*wire.MsgTx, error)

	// Fetch all transactions from the db
	GetAll() ([]*wire.MsgTx, error)

	// Delete a transactions from the db
	Delete(txid *wire.ShaHash) error
}

// Keys provides a database interface for the wallet to save key material, track
// used keys, and manage the look ahead window.
type Keys interface {
	// Put a bip32 key to the database
	Put(scriptPubKey []byte, keyPath KeyPath) error

	// Mark the script as used
	MarkKeyAsUsed(scriptPubKey []byte) error

	// Fetch the last index for the given key purpose
	// The bool should state whether the key has been used or not
	GetLastKeyIndex(purpose KeyPurpose) (int, bool, error)

	// Returns the first unused path for the given purpose
	GetPathForScript(scriptPubKey []byte) (KeyPath, error)

	// Get the first unused index for the given purpose
	GetUnused(purpose KeyPurpose) (int, error)

	// Fetch all key paths
	GetAll() ([]KeyPath, error)

	// Get the number of unused keys following the last used key
	// for each key purpose.
	GetLookaheadWindows() map[KeyPurpose] int
}

type State interface {
	// Put a key/value pair to the database
	Put(key, value string) error

	// Get a value given the key
	Get(key string) (string, error)
}

type ChainState int

const (
	SYNCING = 0
	WAITING = 1
	REORG   = 2
)

type TxStore struct {
	Adrs      []btcutil.Address
	addrMutex *sync.Mutex
	db        Datastore

	Param *chaincfg.Params

	masterPrivKey *hd.ExtendedKey

	chainState ChainState
}

type Utxo struct { // cash money.
	Op wire.OutPoint // where

	// all the info needed to spend
	AtHeight int32  // block height where this tx was confirmed, 0 for unconf
	Value    int64  // higher is better

	ScriptPubkey []byte
}

// Stxo is a utxo that has moved on.
type Stxo struct {
	Utxo        Utxo           // when it used to be a utxo
	SpendHeight int32        // height at which it met its demise
	SpendTxid   wire.ShaHash // the tx that consumed it
}

func NewTxStore(p *chaincfg.Params, db Datastore, masterPrivKey *hd.ExtendedKey) *TxStore {
	txs := new(TxStore)
	txs.Param = p
	txs.db = db
	txs.masterPrivKey = masterPrivKey
	txs.addrMutex = new(sync.Mutex)
	txs.PopulateAdrs()
	return txs
}



// ... or I'm gonna fade away
func (t *TxStore) GimmeFilter() (*bloom.Filter, error) {
	t.PopulateAdrs()

	// get all utxos to add outpoints to filter
	allUtxos, err := t.db.Utxos().GetAll()
	if err != nil {
		return nil, err
	}

	allStxos, err := t.db.Stxos().GetAll()
	if err != nil {
		return nil, err
	}
	t.addrMutex.Lock()
	elem := uint32(len(t.Adrs) + len(allUtxos) + len(allStxos))
	f := bloom.NewFilter(elem, 0, 0.00001, wire.BloomUpdateP2PubkeyOnly)

	// note there could be false positives since we're just looking
	// for the 20 byte PKH without the opcodes.
	for _, a := range t.Adrs { // add 20-byte pubkeyhash
		f.Add(a.ScriptAddress())
	}
	t.addrMutex.Unlock()
	for _, u := range allUtxos {
		f.AddOutPoint(&u.Op)
	}

	for _, s := range allStxos {
		f.AddOutPoint(&s.Utxo.Op)
	}

	return f, nil
}

// GetDoubleSpends takes a transaction and compares it with
// all transactions in the db.  It returns a slice of all txids in the db
// which are double spent by the received tx.
func CheckDoubleSpends(
	argTx *wire.MsgTx, txs []*wire.MsgTx) ([]*wire.ShaHash, error) {

	var dubs []*wire.ShaHash // slice of all double-spent txs
	argTxid := argTx.TxSha()

	for _, compTx := range txs {
		compTxid := compTx.TxSha()
		// check if entire tx is dup
		if argTxid.IsEqual(&compTxid) {
			return nil, fmt.Errorf("tx %s is dup", argTxid.String())
		}
		// not dup, iterate through inputs of argTx
		for _, argIn := range argTx.TxIn {
			// iterate through inputs of compTx
			for _, compIn := range compTx.TxIn {
				if OutPointsEqual(
					argIn.PreviousOutPoint, compIn.PreviousOutPoint) {
					// found double spend
					dubs = append(dubs, &compTxid)
					break // back to argIn loop
				}
			}
		}
	}
	return dubs, nil
}

// need this because before I was comparing pointers maybe?
// so they were the same outpoint but stored in 2 places so false negative?
func OutPointsEqual(a, b wire.OutPoint) bool {
	if !a.Hash.IsEqual(&b.Hash) {
		return false
	}
	return a.Index == b.Index
}

