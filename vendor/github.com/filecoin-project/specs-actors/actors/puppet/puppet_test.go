package puppet_test

import (
	"context"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/puppet"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/filecoin-project/specs-actors/support/mock"
	tutil "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSend(t *testing.T) {

	receiver := tutil.NewIDAddr(t, 100)
	builder := mock.NewBuilder(context.Background(), receiver)

	t.Run("Simple Send", func(t *testing.T) {
		rt := builder.Build(t)
		a := newHarness(t)
		a.constructAndVerify(rt)

		toAddr := tutil.NewIDAddr(t, 101)
		amount := abi.NewTokenAmount(100)
		params := []byte{1, 2, 3, 4, 5}
		methodNum := abi.MethodNum(1)
		sendParams := &puppet.SendParams{
			To:     toAddr,
			Value:  amount,
			Method: methodNum,
			Params: params,
		}

		rt.SetBalance(amount)
		expRet := runtime.CBORBytes([]byte{6, 7, 8, 9, 10})
		rt.ExpectSend(toAddr, 1, runtime.CBORBytes(params), amount, expRet, exitcode.Ok)
		ret := a.puppetSend(rt, sendParams)

		assert.Equal(t, expRet, ret.Return)

	})
}

type actorHarness struct {
	a puppet.Actor
	t testing.TB
}

func newHarness(t testing.TB) *actorHarness {
	return &actorHarness{
		a: puppet.Actor{},
		t: t,
	}
}

func (h *actorHarness) constructAndVerify(rt *mock.Runtime) {
	rt.ExpectValidateCallerAny()
	ret := rt.Call(h.a.Constructor, nil)
	assert.Nil(h.t, ret)
	rt.Verify()
}

func (h *actorHarness) puppetSend(rt *mock.Runtime, params *puppet.SendParams) *puppet.SendReturn {
	rt.ExpectValidateCallerAny()
	ret := rt.Call(h.a.Send, params)
	assert.NotNil(h.t, ret)
	out := ret.(*puppet.SendReturn)
	rt.Verify()
	return out
}
