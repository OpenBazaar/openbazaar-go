package bitcoin

import (
	btc "github.com/btcsuite/btcutil"
	b32 "github.com/tyler-smith/go-bip32"
	"time"
)

// TODO: Build out this interface
type BitcoinWallet interface {
	// Keys
	GetMasterPrivateKey() *b32.Key
	GetMasterPublicKey() *b32.Key
	GetCurrentAddress(purpose KeyPurpose) *btc.AddressPubKeyHash
	GetFreshAddress(purpose KeyPurpose) *btc.AddressPubKeyHash

	// Wallet
	GetBalance() (unconfirmed uint64, confirmed uint64)
}

type KeyPurpose int

const (
	RECEIVING = 0
	CHANGE    = 1
	REFUND    = 2
)

type TransactionState int

const (
	// A (unconfirmed) transaction which does not appear in the best chain
	PENDING   = 0

	// Transaction appears in the best chain
	CONFIRMED = 1

	// We have reason to believe the transaction will never confirm. Either it was double
	// spent or has sat unconfirmed for an unreasonably long period of time.
	DEAD      = 2
)

type TransactionInfo struct {
	Txid            []byte
	Tx              []byte
	Height          int
	State           TransactionState
	Timestamp       time.Time
	Value           int
	ExchangeRate    float64
	ExchangCurrency string
}

type Utxo struct {
	Txid         []byte
	Index        int
	Value        int
	ScriptPubKey []byte
}

type FeeLevel int

const (
	PRIOIRTY = 0
	NORMAL   = 1
	ECONOMIC = 2
)
