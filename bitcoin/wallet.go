package bitcoin

import (
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg"
	btc "github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
)

type BitcoinWallet interface {

	// Returns the type of crytocurrency this wallet implements
	CurrencyCode() string

	// Get the master private key
	MasterPrivateKey() *hd.ExtendedKey

	// Get the master public key
	MasterPublicKey() *hd.ExtendedKey

	// Get the current address for the given purpose
	CurrentAddress(purpose spvwallet.KeyPurpose) btc.Address

	// Get the confirmed and unconfirmed balances
	Balance() (confirmed, unconfirmed int64)

	// Get the height of the blockchain
	ChainTip() uint32

	// Send bitcoins to an external wallet
	Spend(amount int64, addr btc.Address, feeLevel spvwallet.FeeLevel) error

	// Returnt the network parameters
	Params() *chaincfg.Params

	// Add a callback for incoming transactions
	AddTransactionListener(func(spvwallet.TransactionCallback))

	// Cleanly disconnect from the wallet
	Close()
}
