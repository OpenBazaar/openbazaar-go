[![Build Status](https://travis-ci.org/OpenBazaar/spvwallet.svg?branch=master)](https://travis-ci.org/OpenBazaar/spvwallet)
[![Coverage Status](https://coveralls.io/repos/github/OpenBazaar/spvwallet/badge.svg?branch=master)](https://coveralls.io/github/OpenBazaar/spvwallet?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/OpenBazaar/spvwallet)](https://goreportcard.com/report/github.com/OpenBazaar/spvwallet)

# spvwallet

Lightweight p2p SPV wallet and library in Go. It connects directly to the bitcoin p2p network to fetch headers, merkle blocks, and transactions.

It uses a number of utilities from btcsuite but natively handles blockchain and wallet.

Library Usage:
```go
// Create a new config
config := spvwallet.NewDefaultConfig()

// Select network
config.Params = &chaincfg.TestNet3Params

// Select wallet datastore
sqliteDatastore, _ := db.Create(config.RepoPath)
config.DB = sqliteDatastore

// Create the wallet
wallet, _ := spvwallet.NewSPVWallet(config)

// Start it!
go wallet.Start()
```

Easy peasy

The wallet implements the following interface:
```go
type BitcoinWallet interface {

	// Start the wallet
	Start()

	// Return the network parameters
	Params() *chaincfg.Params

	// Returns the type of crytocurrency this wallet implements
	CurrencyCode() string

	// Get the master private key
	MasterPrivateKey() *hd.ExtendedKey

	// Get the master public key
	MasterPublicKey() *hd.ExtendedKey

	// Get the current address for the given purpose
	CurrentAddress(purpose spvwallet.KeyPurpose) btc.Address

	// Returns a fresh address that has never been returned by this function
	NewAddress(purpose spvwallet.KeyPurpose) btc.Address

	// Returns if the wallet has the key for the given address
	HasKey(addr btc.Address) bool

	// Get the confirmed and unconfirmed balances
	Balance() (confirmed, unconfirmed int64)

	// Returns a list of transactions for this wallet
	Transactions() ([]spvwallet.Txn, error)

	// Get info on a specific transaction
	GetTransaction(txid chainhash.Hash) (spvwallet.Txn, error)

	// Get the height of the blockchain
	ChainTip() uint32

	// Get the current fee per byte
	GetFeePerByte(feeLevel spvwallet.FeeLevel) uint64

	// Send bitcoins to an external wallet
	Spend(amount int64, addr btc.Address, feeLevel spvwallet.FeeLevel) (*chainhash.Hash, error)

	// Bump the fee for the given transaction
	BumpFee(txid chainhash.Hash) (*chainhash.Hash, error)

	// Calculates the estimated size of the transaction and returns the total fee for the given feePerByte
	EstimateFee(ins []spvwallet.TransactionInput, outs []spvwallet.TransactionOutput, feePerByte uint64) uint64

	// Build and broadcast a transaction that sweeps all coins from an address. If it is a p2sh multisig, the redeemScript must be included
	SweepAddress(utxos []spvwallet.Utxo, address *btc.Address, key *hd.ExtendedKey, redeemScript *[]byte, feeLevel spvwallet.FeeLevel) (*chainhash.Hash, error)

	// Create a signature for a multisig transaction
	CreateMultisigSignature(ins []spvwallet.TransactionInput, outs []spvwallet.TransactionOutput, key *hd.ExtendedKey, redeemScript []byte, feePerByte uint64) ([]spvwallet.Signature, error)

	// Combine signatures and optionally broadcast
	Multisign(ins []spvwallet.TransactionInput, outs []spvwallet.TransactionOutput, sigs1 []spvwallet.Signature, sigs2 []spvwallet.Signature, redeemScript []byte, feePerByte uint64, broadcast bool) ([]byte, error)

	// Generate a multisig script from public keys
	GenerateMultisigScript(keys []hd.ExtendedKey, threshold int) (addr btc.Address, redeemScript []byte, err error)

	// Add a script to the wallet and get notifications back when coins are received or spent from it
	AddWatchedScript(script []byte) error

	// Add a callback for incoming transactions
	AddTransactionListener(func(spvwallet.TransactionCallback))

	// Use this to re-download merkle blocks in case of missed transactions
	ReSyncBlockchain(fromHeight int32)

	// Return the number of confirmations for a transaction
	GetConfirmations(txid chainhash.Hash) (uint32, error)

	// Cleanly disconnect from the wallet
	Close()
}
```

To create a wallet binary:
```
make install
```

Usage:
```
Usage:
  spvwallet [OPTIONS] <command>

Help Options:
  -h, --help  Show this help message

Available commands:
  addwatchedscript         add a script to watch
  balance                  get the wallet balance
  bumpfee                  bump the tx fee
  chaintip                 return the height of the chain
  createmultisigsignature  create a p2sh multisig signature
  currentaddress           get the current bitcoin address
  estimatefee              estimate the fee for a tx
  getconfirmations         get the number of confirmations for a tx
  getfeeperbyte            get the current bitcoin fee
  gettransaction           get a specific transaction
  haskey                   does key exist
  masterprivatekey         get the wallet's master private key
  masterpublickey          get the wallet's master public key
  multisign                combine multisig signatures
  newaddress               get a new bitcoin address
  peers                    get info about peers
  resyncblockchain         re-download the chain of headers
  spend                    send bitcoins
  start                    start the wallet
  stop                     stop the wallet
  sweepaddress             sweep all coins from an address
  transactions             get a list of transactions
  version                  print the version number
```

Finally a gRPC API is available on port 8234. The same interface is exposed via the API plus a streaming wallet notifier which fires when a new transaction (either incoming or outgoing) is recorded then again when it gains its first confirmation.
