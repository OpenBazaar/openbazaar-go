package wallet

import (
	"errors"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	btc "github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"time"
)

type Wallet interface {

	// Start the wallet
	Start()

	// Return the network parameters
	Params() *chaincfg.Params

	// Returns the type of crytocurrency this wallet implements
	CurrencyCode() string

	// Check if this amount is considered dust
	IsDust(amount int64) bool

	// Get the master private key
	MasterPrivateKey() *hd.ExtendedKey

	// Get the master public key
	MasterPublicKey() *hd.ExtendedKey

	// Generate a child key using the given chaincode. The key is used in multisig transactions.
	// For most implementations this should just be child key 0.
	ChildKey(keyBytes []byte, chaincode []byte, isPrivateKey bool) (*hd.ExtendedKey, error)

	// Get the current address for the given purpose
	CurrentAddress(purpose KeyPurpose) btc.Address

	// Returns a fresh address that has never been returned by this function
	NewAddress(purpose KeyPurpose) btc.Address

	// Parse the address string and return an address interface
	DecodeAddress(addr string) (btc.Address, error)

	// Turn the given output script into an address
	ScriptToAddress(script []byte) (btc.Address, error)

	// Returns if the wallet has the key for the given address
	HasKey(addr btc.Address) bool

	// Get the confirmed and unconfirmed balances
	Balance() (confirmed, unconfirmed int64)

	// Returns a list of transactions for this wallet
	Transactions() ([]Txn, error)

	// Get info on a specific transaction
	GetTransaction(txid chainhash.Hash) (Txn, error)

	// Get the height and best hash of the blockchain
	ChainTip() (uint32, chainhash.Hash)

	// Get the current fee per byte
	GetFeePerByte(feeLevel FeeLevel) uint64

	// Send bitcoins to an external wallet
	Spend(amount int64, addr btc.Address, feeLevel FeeLevel) (*chainhash.Hash, error)

	// Bump the fee for the given transaction
	BumpFee(txid chainhash.Hash) (*chainhash.Hash, error)

	// Calculates the estimated size of the transaction and returns the total fee for the given feePerByte
	EstimateFee(ins []TransactionInput, outs []TransactionOutput, feePerByte uint64) uint64

	// Build a spend transaction for the amount and return the transaction fee
	EstimateSpendFee(amount int64, feeLevel FeeLevel) (uint64, error)

	// Build and broadcast a transaction that sweeps all coins from an address. If it is a p2sh multisig, the redeemScript must be included
	SweepAddress(ins []TransactionInput, address *btc.Address, key *hd.ExtendedKey, redeemScript *[]byte, feeLevel FeeLevel) (*chainhash.Hash, error)

	// Create a signature for a multisig transaction.
	CreateMultisigSignature(ins []TransactionInput, outs []TransactionOutput, key *hd.ExtendedKey, redeemScript []byte, feePerByte uint64) ([]Signature, error)

	// Combine signatures and optionally broadcast
	Multisign(ins []TransactionInput, outs []TransactionOutput, sigs1 []Signature, sigs2 []Signature, redeemScript []byte, feePerByte uint64, broadcast bool) ([]byte, error)

	// Generate a multisig script from public keys. If a timeout is included the returned script should be a timelocked escrow which releases using the timeoutKey.
	GenerateMultisigScript(keys []hd.ExtendedKey, threshold int, timeout time.Duration, timeoutKey *hd.ExtendedKey) (addr btc.Address, redeemScript []byte, err error)

	// Add an address to the wallet and get notifications back when coins are received or spent from it
	AddWatchedAddress(addr btc.Address) error

	// Add a callback for incoming transactions
	AddTransactionListener(func(TransactionCallback))

	// Use this to re-download merkle blocks in case of missed transactions
	ReSyncBlockchain(fromTime time.Time)

	// Return the number of confirmations and the height for a transaction
	GetConfirmations(txid chainhash.Hash) (confirms, atHeight uint32, err error)

	// Cleanly disconnect from the wallet
	Close()
}

type FeeLevel int

const (
	PRIOIRTY FeeLevel = 0
	NORMAL            = 1
	ECONOMIC          = 2
	FEE_BUMP          = 3
)

// The end leaves on the HD wallet have only two possible values. External keys are those given
// to other people for the purpose of receiving transactions. These may include keys used for
// refund addresses. Internal keys are used only by the wallet, primarily for change addresses
// but could also be used for shuffling around UTXOs.
type KeyPurpose int

const (
	EXTERNAL KeyPurpose = 0
	INTERNAL            = 1
)

// This callback is passed to any registered transaction listeners when a transaction is detected
// for the wallet.
type TransactionCallback struct {
	Txid      string
	Outputs   []TransactionOutput
	Inputs    []TransactionInput
	Height    int32
	Timestamp time.Time
	Value     int64
	WatchOnly bool
	BlockTime time.Time
}

type TransactionOutput struct {
	Address btc.Address
	Value   int64
	Index   uint32
}

type TransactionInput struct {
	OutpointHash  []byte
	OutpointIndex uint32
	LinkedAddress btc.Address
	Value         int64
}

// OpenBazaar uses p2sh addresses for escrow. This object can be used to store a record of a
// transaction going into or out of such an address. Incoming transactions should have a positive
// value and be market as spent when the UXTO is spent. Outgoing transactions should have a
// negative value. The spent field isn't relevant for outgoing transactions.
type TransactionRecord struct {
	Txid      string
	Index     uint32
	Value     int64
	Address   string
	Spent     bool
	Timestamp time.Time
}

// This object contains a single signature for a multisig transaction. InputIndex specifies
// the index for which this signature applies.
type Signature struct {
	InputIndex uint32
	Signature  []byte
}

// Errors
var (
	ErrorInsuffientFunds error = errors.New("Insuffient funds")
	ErrorDustAmount      error = errors.New("Amount is below network dust treshold")
)
