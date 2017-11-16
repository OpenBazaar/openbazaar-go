package bitcoincash

// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

/* Copied here from a btcd internal package*/

import (
	"github.com/btcsuite/btcd/wire"
)

// Worst case script and input/output size estimates.
const (
	// RedeemP2PKHSigScriptSize is the worst case (largest) serialize size
	// of a transaction input script that redeems a compressed P2PKH output.
	// It is calculated as:
	//
	//   - OP_DATA_73
	//   - 72 bytes DER signature + 1 byte sighash
	//   - OP_DATA_33
	//   - 33 bytes serialized compressed pubkey
	RedeemP2PKHSigScriptSize = 1 + 73 + 1 + 33

	// RedeemP2SHMultisigSigScriptSize is the worst case (largest) serialize size
	// of a transaction input script that redeems a 2 of 3 P2SH multisig output with compressed keys.
	// It is calculated as:
	//
	//   - OP_0
	//   - OP_DATA_72
	//   - 72 bytes DER signature
	//   - OP_DATA_72
	//   - 72 bytes DER signature
	//   - OP_PUSHDATA
	//   - OP_2
	//   - OP_DATA_33
	//   - 33 bytes serialized compressed pubkey
	//   - OP_DATA_33
	//   - 33 bytes serialized compressed pubkey
	//   - OP_DATA_33
	//   - 33 bytes serialized compressed pubkey
	//   - OP3
	//   - OP_CHECKMULTISIG
	RedeemP2SH2of3MultisigSigScriptSize = 1 + 1 + 72 + 1 + 72 + 1 + 1 + 1 + 33 + 1 + 33 + 1 + 33 + 1 + 1

	// RedeemP2SH1of2MultisigSigScriptSize is the worst case (largest) serialize size
	// of a transaction input script that redeems a 1 of 2 P2SH multisig output with compressed keys.
	// It is calculated as:
	//
	//   - OP_0
	//   - OP_DATA_72
	//   - 72 bytes DER signature
	//   - OP_PUSHDATA
	//   - OP_1
	//   - OP_DATA_33
	//   - 33 bytes serialized compressed pubkey
	//   - OP_DATA_33
	//   - 33 bytes serialized compressed pubkey
	//   - OP2
	//   - OP_CHECKMULTISIG
	RedeemP2SH1of2MultisigSigScriptSize = 1 + 1 + 72 + 1 + 1 + 1 + 33 + 1 + 33 + 1 + 1

	// RedeemP2SHMultisigTimelock1SigScriptSize is the worst case (largest) serialize size
	// of a transaction input script that redeems a compressed P2SH timelocked multisig using the timeout.
	// It is calculated as:
	//
	//   - OP_DATA_72
	//   - 72 bytes DER signature
	//   - OP_0
	//   - OP_PUSHDATA
	//   - OP_IF
	//   - OP_2
	//   - OP_DATA_33
	//   - 33 bytes serialized compressed pubkey
	//   - OP_DATA_33
	//   - 33 bytes serialized compressed pubkey
	//   - OP_DATA_33
	//   - 33 bytes serialized compressed pubkey
	//   - OP3
	//   - OP_CHECKMULTISIG
	//   - OP_ELSE
	//   - OP_PUSHDATA
	//   - 2 byte block height
	//   - OP_CHECKSEQUENCEVERIFY
	//   - OP_DROP
	//   - OP_DATA_33
	//   - 33 bytes serialized compressed pubkey
	//   - OP_CHECKSIG
	//   - OP_ENDIF
	RedeemP2SHMultisigTimelock1SigScriptSize = 1 + 72 + 1 + 1 + 1 + 1 + 1 + 33 + 1 + 33 + 1 + 33 + 1 + 1 + 1 + 1 + 2 + 1 + 1 + 1 + 33 + 1 + 1

	// RedeemP2SHMultisigTimelock2SigScriptSize is the worst case (largest) serialize size
	// of a transaction input script that redeems a compressed P2SH timelocked multisig without using the timeout.
	// It is calculated as:
	//
	//   - OP_0
	//   - OP_DATA_72
	//   - 72 bytes DER signature
	//   - OP_DATA_72
	//   - 72 bytes DER signature
	//   - OP_1
	//   - OP_PUSHDATA
	//   - OP_IF
	//   - OP_2
	//   - OP_DATA_33
	//   - 33 bytes serialized compressed pubkey
	//   - OP_DATA_33
	//   - 33 bytes serialized compressed pubkey
	//   - OP_DATA_33
	//   - 33 bytes serialized compressed pubkey
	//   - OP3
	//   - OP_CHECKMULTISIG
	//   - OP_ELSE
	//   - OP_PUSHDATA
	//   - 2 byte block height
	//   - OP_CHECKSEQUENCEVERIFY
	//   - OP_DROP
	//   - OP_DATA_33
	//   - 33 bytes serialized compressed pubkey
	//   - OP_CHECKSIG
	//   - OP_ENDIF
	RedeemP2SHMultisigTimelock2SigScriptSize = 1 + 1 + 72 + +1 + 72 + 1 + 1 + 1 + 1 + 1 + 33 + 1 + 33 + 1 + 33 + 1 + 1 + 1 + 1 + 2 + 1 + 1 + 1 + 33 + 1 + 1

	// P2PKHPkScriptSize is the size of a transaction output script that
	// pays to a compressed pubkey hash.  It is calculated as:
	//
	//   - OP_DUP
	//   - OP_HASH160
	//   - OP_DATA_20
	//   - 20 bytes pubkey hash
	//   - OP_EQUALVERIFY
	//   - OP_CHECKSIG
	P2PKHPkScriptSize = 1 + 1 + 1 + 20 + 1 + 1

	// RedeemP2PKHInputSize is the worst case (largest) serialize size of a
	// transaction input redeeming a compressed P2PKH output.  It is
	// calculated as:
	//
	//   - 32 bytes previous tx
	//   - 4 bytes output index
	//   - 1 byte script len
	//   - signature script
	//   - 4 bytes sequence
	RedeemP2PKHInputSize = 32 + 4 + 1 + RedeemP2PKHSigScriptSize + 4

	// RedeemP2SH2of3MultisigInputSize is the worst case (largest) serialize size of a
	// transaction input redeeming a compressed P2SH 2 of 3 multisig output.  It is
	// calculated as:
	//
	//   - 32 bytes previous tx
	//   - 4 bytes output index
	//   - 1 byte script len
	//   - 4 bytes sequence
	///  - witness discounted signature script
	RedeemP2SH2of3MultisigInputSize = 32 + 4 + 1 + 4 + (RedeemP2SH2of3MultisigSigScriptSize / 4)

	// RedeemP2SH1of2MultisigInputSize is the worst case (largest) serialize size of a
	// transaction input redeeming a compressed P2SH 2 of 3 multisig output.  It is
	// calculated as:
	//
	//   - 32 bytes previous tx
	//   - 4 bytes output index
	//   - 1 byte script len
	//   - 4 bytes sequence
	///  - witness discounted signature script
	RedeemP2SH1of2MultisigInputSize = 32 + 4 + 1 + 4 + (RedeemP2SH1of2MultisigSigScriptSize / 4)

	// RedeemP2SHMultisigTimelock1InputSize is the worst case (largest) serialize size of a
	// transaction input redeeming a compressed p2sh timelocked multig output with using the timeout.  It is
	// calculated as:
	//
	//   - 32 bytes previous tx
	//   - 4 bytes output index
	//   - 1 byte script len
	//   - 4 bytes sequence
	///  - witness discounted signature script
	RedeemP2SHMultisigTimelock1InputSize = 32 + 4 + 1 + 4 + (RedeemP2SHMultisigTimelock1SigScriptSize / 4)

	// RedeemP2SHMultisigTimelock2InputSize is the worst case (largest) serialize size of a
	// transaction input redeeming a compressed P2SH timelocked multisig output without using the timeout.  It is
	// calculated as:
	//
	//   - 32 bytes previous tx
	//   - 4 bytes output index
	//   - 1 byte script len
	//   - 4 bytes sequence
	///  - witness discounted signature script
	RedeemP2SHMultisigTimelock2InputSize = 32 + 4 + 1 + 4 + (RedeemP2SHMultisigTimelock2SigScriptSize / 4)

	// P2PKHOutputSize is the serialize size of a transaction output with a
	// P2PKH output script.  It is calculated as:
	//
	//   - 8 bytes output value
	//   - 1 byte compact int encoding value 25
	//   - 25 bytes P2PKH output script
	P2PKHOutputSize = 8 + 1 + P2PKHPkScriptSize
)

type InputType int

const (
	P2PKH InputType = iota
	P2SH_1of2_Multisig
	P2SH_2of3_Multisig
	P2SH_Multisig_Timelock_1Sig
	P2SH_Multisig_Timelock_2Sigs
)

// EstimateSerializeSize returns a worst case serialize size estimate for a
// signed transaction that spends inputCount number of compressed P2PKH outputs
// and contains each transaction output from txOuts.  The estimated size is
// incremented for an additional P2PKH change output if addChangeOutput is true.
func EstimateSerializeSize(inputCount int, txOuts []*wire.TxOut, addChangeOutput bool, inputType InputType) int {
	changeSize := 0
	outputCount := len(txOuts)
	if addChangeOutput {
		changeSize = P2PKHOutputSize
		outputCount++
	}

	var redeemScriptSize int
	switch inputType {
	case P2PKH:
		redeemScriptSize = RedeemP2PKHInputSize
	case P2SH_1of2_Multisig:
		redeemScriptSize = RedeemP2SH1of2MultisigInputSize
	case P2SH_2of3_Multisig:
		redeemScriptSize = RedeemP2SH2of3MultisigInputSize
	case P2SH_Multisig_Timelock_1Sig:
		redeemScriptSize = RedeemP2SHMultisigTimelock1InputSize
	case P2SH_Multisig_Timelock_2Sigs:
		redeemScriptSize = RedeemP2SHMultisigTimelock2InputSize
	}

	// 10 additional bytes are for version, locktime, and segwit flags
	return 10 + wire.VarIntSerializeSize(uint64(inputCount)) +
		wire.VarIntSerializeSize(uint64(outputCount)) +
		inputCount*redeemScriptSize +
		SumOutputSerializeSizes(txOuts) +
		changeSize
}

// SumOutputSerializeSizes sums up the serialized size of the supplied outputs.
func SumOutputSerializeSizes(outputs []*wire.TxOut) (serializeSize int) {
	for _, txOut := range outputs {
		serializeSize += txOut.SerializeSize()
	}
	return serializeSize
}
