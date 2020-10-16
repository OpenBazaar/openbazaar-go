package util

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"testing"
)

func TestOutPointsEqual(t *testing.T) {
	h1, err := chainhash.NewHashFromStr("6f7a58ad92702601fcbaac0e039943a384f5274a205c16bb8bbab54f9ea2fbad")
	if err != nil {
		t.Error(err)
	}
	op := wire.NewOutPoint(h1, 0)
	h2, err := chainhash.NewHashFromStr("a0d4cbcd8d0694e1132400b5e114b31bc3e0d8a2ac26e054f78727c95485b528")
	op2 := wire.NewOutPoint(h2, 0)
	if err != nil {
		t.Error(err)
	}
	if !OutPointsEqual(*op, *op) {
		t.Error("Failed to detect equal outpoints")
	}
	if OutPointsEqual(*op, *op2) {
		t.Error("Incorrectly returned equal outpoints")
	}
}
