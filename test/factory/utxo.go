package factory

import (
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func NewUtxo() wallet.Utxo {
	sh1, err := chainhash.NewHashFromStr("e941e1c32b3dd1a68edc3af9f7fe711f35aaca60f758c2dd49561e45ca2c41c0")
	if err != nil {
		panic(err)
	}
	outpoint := wire.NewOutPoint(sh1, 0)
	return wallet.Utxo{
		Op:           *outpoint,
		AtHeight:     300000,
		Value:        100000000,
		ScriptPubkey: []byte("scriptpubkey"),
		WatchOnly:    false,
	}
}
