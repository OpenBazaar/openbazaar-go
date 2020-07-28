package verifreg_test

import (
	"testing"

	"github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	"github.com/filecoin-project/specs-actors/support/mock"
)

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, verifreg.Actor{})
}
