package bitcoin

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/OpenBazaar/spvwallet"
	btc "github.com/btcsuite/btcutil"
	b32 "github.com/tyler-smith/go-bip32"
)

type BitcoinWallet interface {

	// Returns the type of crytocurrency this wallet implements
	CurrencyCode() string

	// Get the master private key
	MasterPrivateKey() *b32.Key

	// Get the master public key
	MasterPublicKey() *b32.Key

	// Get the current address for the given purpose
	CurrentAddress(purpose spvwallet.KeyPurpose) *btc.AddressPubKeyHash

	// Get the confirmed and unconfirmed balances
	Balance() (confirmed, unconfirmed int64)

	// Send bitcoins to an external wallet
	Spend(amount int64, addr btc.Address, feeLevel spvwallet.FeeLevel) error

	// Returnt the network parameters
	Params() *chaincfg.Params
}
