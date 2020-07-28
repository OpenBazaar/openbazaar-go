package init

import (
	addr "github.com/filecoin-project/go-address"
	cid "github.com/ipfs/go-cid"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	runtime "github.com/filecoin-project/specs-actors/actors/runtime"
	exitcode "github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	autil "github.com/filecoin-project/specs-actors/actors/util"
	adt "github.com/filecoin-project/specs-actors/actors/util/adt"
)

// The init actor uniquely has the power to create new actors.
// It maintains a table resolving pubkey and temporary actor addresses to the canonical ID-addresses.
type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.Exec,
	}
}

var _ abi.Invokee = Actor{}

type ConstructorParams struct {
	NetworkName string
}

func (a Actor) Constructor(rt runtime.Runtime, params *ConstructorParams) *adt.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)
	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to construct state: %v", err)
	}

	st := ConstructState(emptyMap, params.NetworkName)
	rt.State().Create(st)
	return nil
}

type ExecParams struct {
	CodeCID           cid.Cid
	ConstructorParams []byte
}

type ExecReturn struct {
	IDAddress     addr.Address // The canonical ID-based address for the actor.
	RobustAddress addr.Address // A more expensive but re-org-safe address for the newly created actor.
}

func (a Actor) Exec(rt runtime.Runtime, params *ExecParams) *ExecReturn {
	rt.ValidateImmediateCallerAcceptAny()
	callerCodeCID, ok := rt.GetActorCodeCID(rt.Message().Caller())
	autil.AssertMsg(ok, "no code for actor at %s", rt.Message().Caller())
	if !canExec(callerCodeCID, params.CodeCID) {
		rt.Abortf(exitcode.ErrForbidden, "caller type %v cannot exec actor type %v", callerCodeCID, params.CodeCID)
	}

	// Compute a re-org-stable address.
	// This address exists for use by messages coming from outside the system, in order to
	// stably address the newly created actor even if a chain re-org causes it to end up with
	// a different ID.
	uniqueAddress := rt.NewActorAddress()

	// Allocate an ID for this actor.
	// Store mapping of pubkey or actor address to actor ID
	var st State
	idAddr := rt.State().Transaction(&st, func() interface{} {
		idAddr, err := st.MapAddressToNewID(adt.AsStore(rt), uniqueAddress)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "exec failed: %v", err)
		}
		return idAddr
	}).(addr.Address)

	// Create an empty actor.
	rt.CreateActor(params.CodeCID, idAddr)

	// Invoke constructor.
	_, code := rt.Send(idAddr, builtin.MethodConstructor, runtime.CBORBytes(params.ConstructorParams), rt.Message().ValueReceived())
	builtin.RequireSuccess(rt, code, "constructor failed")

	return &ExecReturn{idAddr, uniqueAddress}
}

func canExec(callerCodeID cid.Cid, execCodeID cid.Cid) bool {
	switch execCodeID {
	case builtin.StorageMinerActorCodeID:
		if callerCodeID == builtin.StoragePowerActorCodeID {
			return true
		}
		return false
	case builtin.PaymentChannelActorCodeID, builtin.MultisigActorCodeID:
		return true
	default:
		return false
	}
}
