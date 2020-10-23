package litecoin

// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

/* Copied here from a btcd internal package*/

import (
	"bytes"
	"encoding/hex"
	"github.com/btcsuite/btcd/wire"
	"testing"
)

const (
	p2pkhScriptSize = P2PKHPkScriptSize
	p2shScriptSize  = 23
)

func makeInts(value int, n int) []int {
	v := make([]int, n)
	for i := range v {
		v[i] = value
	}
	return v
}

func TestEstimateSerializeSize(t *testing.T) {
	tests := []struct {
		InputCount           int
		OutputScriptLengths  []int
		AddChangeOutput      bool
		ExpectedSizeEstimate int
	}{
		0: {1, []int{}, false, 161},
		1: {1, []int{p2pkhScriptSize}, false, 195},
		2: {1, []int{}, true, 195},
		3: {1, []int{p2pkhScriptSize}, true, 229},
		4: {1, []int{p2shScriptSize}, false, 193},
		5: {1, []int{p2shScriptSize}, true, 227},

		6:  {2, []int{}, false, 310},
		7:  {2, []int{p2pkhScriptSize}, false, 344},
		8:  {2, []int{}, true, 344},
		9:  {2, []int{p2pkhScriptSize}, true, 378},
		10: {2, []int{p2shScriptSize}, false, 342},
		11: {2, []int{p2shScriptSize}, true, 376},

		// 0xfd is discriminant for 16-bit compact ints, compact int
		// total size increases from 1 byte to 3.
		12: {1, makeInts(p2pkhScriptSize, 0xfc), false, 8729},
		13: {1, makeInts(p2pkhScriptSize, 0xfd), false, 8729 + P2PKHOutputSize + 2},
		14: {1, makeInts(p2pkhScriptSize, 0xfc), true, 8729 + P2PKHOutputSize + 2},
		15: {0xfc, []int{}, false, 37560},
		16: {0xfd, []int{}, false, 37560 + RedeemP2PKHInputSize + 2},
	}
	for i, test := range tests {
		outputs := make([]*wire.TxOut, 0, len(test.OutputScriptLengths))
		for _, l := range test.OutputScriptLengths {
			outputs = append(outputs, &wire.TxOut{PkScript: make([]byte, l)})
		}
		actualEstimate := EstimateSerializeSize(test.InputCount, outputs, test.AddChangeOutput, P2PKH)
		if actualEstimate != test.ExpectedSizeEstimate {
			t.Errorf("Test %d: Got %v: Expected %v", i, actualEstimate, test.ExpectedSizeEstimate)
		}
	}
}

func TestSumOutputSerializeSizes(t *testing.T) {
	testTx := "0100000001066b78efa7d66d271cae6d6eb799e1d10953fb1a4a760226cc93186d52b55613010000006a47304402204e6c32cc214c496546c3277191ca734494fe49fed0af1d800db92fed2021e61802206a14d063b67f2f1c8fc18f9e9a5963fe33e18c549e56e3045e88b4fc6219be11012103f72d0a11727219bff66b8838c3c5e1c74a5257a325b0c84247bd10bdb9069e88ffffffff0200c2eb0b000000001976a914426e80ad778792e3e19c20977fb93ec0591e1a3988ac35b7cb59000000001976a914e5b6dc0b297acdd99d1a89937474df77db5743c788ac00000000"
	txBytes, err := hex.DecodeString(testTx)
	if err != nil {
		t.Error(err)
		return
	}
	r := bytes.NewReader(txBytes)
	msgTx := wire.NewMsgTx(1)
	msgTx.BtcDecode(r, 1, wire.WitnessEncoding)
	if SumOutputSerializeSizes(msgTx.TxOut) != 68 {
		t.Error("SumOutputSerializeSizes returned incorrect value")
	}

}
