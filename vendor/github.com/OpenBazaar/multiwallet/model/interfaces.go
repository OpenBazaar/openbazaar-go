package model

import "github.com/btcsuite/btcutil"

type APIClient interface {

	// Start up the API service
	Start() error

	// Get info about the server
	GetInfo() (*Info, error)

	// For a given txid get back the transaction metadata
	GetTransaction(txid string) (*Transaction, error)

	// For a given txid get back the full transaction bytes
	GetRawTransaction(txid string) ([]byte, error)

	// Get back all the transactions for the given list of addresses
	GetTransactions(addrs []btcutil.Address) ([]Transaction, error)

	// Get back all spendable UTXOs for the given list of addresses
	GetUtxos(addrs []btcutil.Address) ([]Utxo, error)

	// Returns a chan which fires on each new block
	BlockNotify() <-chan Block

	// Returns a chan which fires whenever a new transaction is received or
	// when an existing transaction confirms for all addresses the API is listening on.
	TransactionNotify() <-chan Transaction

	// Listen for events on these addresses. Results are returned to TransactionNotify()
	ListenAddresses(addrs ...btcutil.Address)

	// Broadcast a transaction to the network
	Broadcast(tx []byte) (string, error)

	// Get info on the current chain tip
	GetBestBlock() (*Block, error)

	// Estimate the fee required for a transaction
	EstimateFee(nBlocks int) (int, error)

	// Close all connections and shutdown
	Close()
}

type SocketClient interface {

	// Set callback for method
	On(method string, callback interface{}) error

	// Listen on method
	Emit(method string, args []interface{}) error

	// Close the socket connection
	Close()
}
