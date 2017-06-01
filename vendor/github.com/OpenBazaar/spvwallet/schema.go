package spvwallet

import "time"

// TODO: Eventually we will like to move of this file to a separate interface repo which this wallet
// TODO: and others (such as the openbazaar-go bitcoind wallet) can share.

// Selecting the correct fee for a transaction can be difficult for end users. We try to simplify
// this by create three generic fee levels (the exact data for which can either be hardcoded or
// fetched via API).
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
	Txid      []byte
	Outputs   []TransactionOutput
	Inputs    []TransactionInput
	Height    int32
	Timestamp time.Time
	Value     int64
	WatchOnly bool
}

type TransactionOutput struct {
	ScriptPubKey []byte
	Value        int64
	Index        uint32
}

type TransactionInput struct {
	OutpointHash       []byte
	OutpointIndex      uint32
	LinkedScriptPubKey []byte
	Value              int64
}

// OpenBazaar uses p2sh addresses for escrow. This object can be used to store a record of a
// transaction going into or out of such an address. Incoming transactions should have a positive
// value and be market as spent when the UXTO is spent. Outgoing transactions should have a
// negative value. The spent field isn't relevant for outgoing transactions.
type TransactionRecord struct {
	Txid         string
	Index        uint32
	Value        int64
	ScriptPubKey string
	Spent        bool
	Timestamp    time.Time
}

// This object contains a single signature for a multisig transaction. InputIndex specifies
// the index for which this signature applies.
type Signature struct {
	InputIndex uint32
	Signature  []byte
}
