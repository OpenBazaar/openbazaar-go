package builtin

import (
	addr "github.com/filecoin-project/go-address"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	runtime "github.com/filecoin-project/specs-actors/actors/runtime"
	exitcode "github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	autil "github.com/filecoin-project/specs-actors/actors/util"
)

///// Code shared by multiple built-in actors. /////

// Aborts with an ErrIllegalArgument if predicate is not true.
func RequireParam(rt runtime.Runtime, predicate bool, msg string, args ...interface{}) {
	if !predicate {
		rt.Abortf(exitcode.ErrIllegalArgument, msg, args...)
	}
}

// Propagates a failed send by aborting the current method with the same exit code.
func RequireSuccess(rt runtime.Runtime, e exitcode.ExitCode, msg string, args ...interface{}) {
	if !e.IsSuccess() {
		rt.Abortf(e, msg, args...)
	}
}

// Aborts with a formatted message if err is not nil.
// The provided message will be suffixed by ": %s" and the provided args suffixed by the err.
func RequireNoErr(rt runtime.Runtime, err error, code exitcode.ExitCode, msg string, args ...interface{}) {
	if err != nil {
		newMsg := msg + ": %s"
		newArgs := append(args, err)
		rt.Abortf(code, newMsg, newArgs...)
	}
}

func RequestMinerControlAddrs(rt runtime.Runtime, minerAddr addr.Address) (ownerAddr addr.Address, workerAddr addr.Address) {
	ret, code := rt.Send(minerAddr, MethodsMiner.ControlAddresses, nil, abi.NewTokenAmount(0))
	RequireSuccess(rt, code, "failed fetching control addresses")
	var addrs MinerAddrs
	autil.AssertNoError(ret.Into(&addrs))

	return addrs.Owner, addrs.Worker
}

// This type duplicates the Miner.ControlAddresses return type, to work around a circular dependency between actors.
type MinerAddrs struct {
	Owner  addr.Address
	Worker addr.Address
}

type ConfirmSectorProofsParams struct {
	Sectors []abi.SectorNumber
}
