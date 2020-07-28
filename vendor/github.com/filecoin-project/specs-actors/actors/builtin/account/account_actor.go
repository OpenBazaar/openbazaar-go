package account

import (
	addr "github.com/filecoin-project/go-address"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	vmr "github.com/filecoin-project/specs-actors/actors/runtime"
	exitcode "github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	adt "github.com/filecoin-project/specs-actors/actors/util/adt"
)

type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		1: a.Constructor,
		2: a.PubkeyAddress,
	}
}

var _ abi.Invokee = Actor{}

type State struct {
	Address addr.Address
}

func (a Actor) Constructor(rt vmr.Runtime, address *addr.Address) *adt.EmptyValue {
	// Account actors are created implicitly by sending a message to a pubkey-style address.
	// This constructor is not invoked by the InitActor, but by the system.
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)
	switch address.Protocol() {
	case addr.SECP256K1:
	case addr.BLS:
		break // ok
	default:
		rt.Abortf(exitcode.ErrIllegalArgument, "address must use BLS or SECP protocol, got %v", address.Protocol())
	}
	st := State{Address: *address}
	rt.State().Create(&st)
	return nil
}

// Fetches the pubkey-type address from this actor.
func (a Actor) PubkeyAddress(rt vmr.Runtime, _ *adt.EmptyValue) addr.Address {
	rt.ValidateImmediateCallerAcceptAny()
	var st State
	rt.State().Readonly(&st)
	return st.Address
}
