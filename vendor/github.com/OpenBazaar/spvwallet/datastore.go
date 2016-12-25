package spvwallet

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type Datastore interface {
	Utxos() Utxos
	Stxos() Stxos
	Txns() Txns
	Keys() Keys
	WatchedScripts() WatchedScripts
}

type Utxos interface {
	// Put a utxo to the database
	Put(utxo Utxo) error

	// Fetch all utxos from the db
	GetAll() ([]Utxo, error)

	// Make a utxo unspendable
	Freeze(utxo Utxo) error

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
	Get(txid chainhash.Hash) (*wire.MsgTx, error)

	// Fetch all transactions from the db
	GetAll() ([]*wire.MsgTx, error)

	// Delete a transactions from the db
	Delete(txid *chainhash.Hash) error
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
	GetLookaheadWindows() map[KeyPurpose]int
}

type WatchedScripts interface {
	// Add a script to watch
	Put(scriptPubKey []byte) error

	// Return all watched scripts
	GetAll() ([][]byte, error)

	// Delete a watched script
	Delete(scriptPubKey []byte) error
}

type Utxo struct {
	// Previous txid and output index
	Op wire.OutPoint

	// Block height where this tx was confirmed, 0 for unconfirmed
	AtHeight int32

	// The higher the better
	Value int64

	// Output script
	ScriptPubkey []byte

	// If true this utxo will not be selected for spending. The primary
	// purpose is track multisig UTXOs which must have separate handling
	// to spend.
	Freeze bool
}

type Stxo struct {
	// When it used to be a UTXO
	Utxo Utxo

	// The height at which it met its demise
	SpendHeight int32

	// The tx that consumed it
	SpendTxid chainhash.Hash
}
