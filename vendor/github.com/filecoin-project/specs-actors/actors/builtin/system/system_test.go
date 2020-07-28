package system_test

import (
	"testing"

	"github.com/filecoin-project/specs-actors/actors/builtin/system"
	"github.com/filecoin-project/specs-actors/support/mock"
)

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, system.Actor{})
}
