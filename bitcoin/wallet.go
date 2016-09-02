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

	// Check if we have sufficient funds to make a tx
	CheckSuffientFunds(amount int64, feeLevel spvwallet.FeeLevel) error

	// Send bitcoins to an external wallet
	Spend(amount int64, addr btc.Address, feeLevel spvwallet.FeeLevel) error

	// Returns the raw bytes for a signed transaction suitable for sending to a peer out of band
	// The uxtos this transaction spends should be frozen to prevent double spending
	ExportRawTx(amount int64, addr btc.Address, feeLevel spvwallet.FeeLevel) ([]byte, error)

	// Broadcast a raw tx
	BroadcastRawTx(tx []byte) error

	// Returnt the network parameters
	Params() *chaincfg.Params

	// Add a callback for incoming transactions
	AddTransactionListener(func(addr btc.Address, amount int64, incoming bool))
}
