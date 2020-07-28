package system

import (
	abi "github.com/filecoin-project/specs-actors/actors/abi"
	builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	runtime "github.com/filecoin-project/specs-actors/actors/runtime"
	adt "github.com/filecoin-project/specs-actors/actors/util/adt"
)

type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
	}
}

var _ abi.Invokee = Actor{}

func (a Actor) Constructor(rt runtime.Runtime, _ *adt.EmptyValue) *adt.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	rt.State().Create(&State{})
	return nil
}

type State struct{}
