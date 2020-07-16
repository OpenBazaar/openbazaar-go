package test

import (
	"testing"

	"github.com/OpenBazaar/multiwallet/model"
)

func ValidateTransaction(tx, expectedTx model.Transaction, t *testing.T) {
	if tx.Txid != expectedTx.Txid {
		t.Error("Returned invalid transaction")
	}
	if tx.Version != expectedTx.Version {
		t.Error("Returned invalid transaction")
	}
	if tx.Locktime != expectedTx.Locktime {
		t.Error("Returned invalid transaction")
	}
	if tx.Time != expectedTx.Time {
		t.Error("Returned invalid transaction")
	}
	if tx.BlockHash != expectedTx.BlockHash {
		t.Error("Returned invalid transaction")
	}
	if tx.BlockHeight != expectedTx.BlockHeight {
		t.Error("Returned invalid transaction")
	}
	if tx.Confirmations != expectedTx.Confirmations {
		t.Error("Returned invalid transaction")
	}
	if len(tx.Inputs) != 1 {
		t.Error("Returned incorrect number of inputs")
		return
	}
	if tx.Inputs[0].Txid != expectedTx.Inputs[0].Txid {
		t.Error("Returned invalid transaction")
	}
	if tx.Inputs[0].Value != 0.04294455 {
		t.Error("Returned invalid transaction")
	}
	if tx.Inputs[0].Satoshis != expectedTx.Inputs[0].Satoshis {
		t.Error("Returned invalid transaction")
	}
	if tx.Inputs[0].Addr != expectedTx.Inputs[0].Addr {
		t.Error("Returned invalid transaction")
	}
	if tx.Inputs[0].Sequence != expectedTx.Inputs[0].Sequence {
		t.Error("Returned invalid transaction")
	}
	if tx.Inputs[0].Vout != expectedTx.Inputs[0].Vout {
		t.Error("Returned invalid transaction")
	}
	if tx.Inputs[0].ScriptSig.Hex != expectedTx.Inputs[0].ScriptSig.Hex {
		t.Error("Returned invalid transaction")
	}

	if len(tx.Outputs) != 2 {
		t.Error("Returned incorrect number of outputs")
		return
	}
	if tx.Outputs[0].Value != 0.01398175 {
		t.Error("Returned invalid transaction")
	}
	if tx.Outputs[0].ScriptPubKey.Hex != expectedTx.Outputs[0].ScriptPubKey.Hex {
		t.Error("Returned invalid transaction")
	}
	if tx.Outputs[0].ScriptPubKey.Type != expectedTx.Outputs[0].ScriptPubKey.Type {
		t.Error("Returned invalid transaction")
	}
	if tx.Outputs[0].ScriptPubKey.Addresses[0] != expectedTx.Outputs[0].ScriptPubKey.Addresses[0] {
		t.Error("Returned invalid transaction")
	}
	if tx.Outputs[1].Value != 0.02717080 {
		t.Error("Returned invalid transaction")
	}
	if tx.Outputs[1].ScriptPubKey.Hex != expectedTx.Outputs[1].ScriptPubKey.Hex {
		t.Error("Returned invalid transaction")
	}
	if tx.Outputs[1].ScriptPubKey.Type != expectedTx.Outputs[1].ScriptPubKey.Type {
		t.Error("Returned invalid transaction")
	}
	if tx.Outputs[1].ScriptPubKey.Addresses[0] != expectedTx.Outputs[1].ScriptPubKey.Addresses[0] {
		t.Error("Returned invalid transaction")
	}
}
