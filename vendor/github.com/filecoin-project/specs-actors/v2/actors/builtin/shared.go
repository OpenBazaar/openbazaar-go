package builtin

import (
	"bytes"
	"fmt"
	"io"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"

	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime"
)

///// Code shared by multiple built-in actors. /////

type BigFrac struct {
	Numerator   big.Int
	Denominator big.Int
}

// Wraps already-serialized bytes as CBOR-marshalable.
type CBORBytes []byte

func (b CBORBytes) MarshalCBOR(w io.Writer) error {
	_, err := w.Write(b)
	return err
}

func (b *CBORBytes) UnmarshalCBOR(r io.Reader) error {
	var c bytes.Buffer
	_, err := c.ReadFrom(r)
	*b = c.Bytes()
	return err
}

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
func RequireNoErr(rt runtime.Runtime, err error, defaultExitCode exitcode.ExitCode, msg string, args ...interface{}) {
	if err != nil {
		newMsg := msg + ": %s"
		newArgs := append(args, err)
		code := exitcode.Unwrap(err, defaultExitCode)
		rt.Abortf(code, newMsg, newArgs...)
	}
}

func RequestMinerControlAddrs(rt runtime.Runtime, minerAddr addr.Address) (ownerAddr addr.Address, workerAddr addr.Address, controlAddrs []addr.Address) {
	var addrs MinerAddrs
	code := rt.Send(minerAddr, MethodsMiner.ControlAddresses, nil, abi.NewTokenAmount(0), &addrs)
	RequireSuccess(rt, code, "failed fetching control addresses")

	return addrs.Owner, addrs.Worker, addrs.ControlAddrs
}

// This type duplicates the Miner.ControlAddresses return type, to work around a circular dependency between actors.
type MinerAddrs struct {
	Owner        addr.Address
	Worker       addr.Address
	ControlAddrs []addr.Address
}

// Note: we could move this alias back to the mutually-importing packages that use it, now that they
// can instead both alias the v0 version.
//type ConfirmSectorProofsParams struct {
//	Sectors []abi.SectorNumber
//}
type ConfirmSectorProofsParams = builtin0.ConfirmSectorProofsParams

// ResolveToIDAddr resolves the given address to it's ID address form.
// If an ID address for the given address dosen't exist yet, it tries to create one by sending a zero balance to the given address.
func ResolveToIDAddr(rt runtime.Runtime, address addr.Address) (addr.Address, error) {
	// if we are able to resolve it to an ID address, return the resolved address
	idAddr, found := rt.ResolveAddress(address)
	if found {
		return idAddr, nil
	}

	// send 0 balance to the account so an ID address for it is created and then try to resolve
	code := rt.Send(address, MethodSend, nil, abi.NewTokenAmount(0), &Discard{})
	if !code.IsSuccess() {
		return address, code.Wrapf("failed to send zero balance to address %v", address)
	}

	// now try to resolve it to an ID address -> fail if not possible
	idAddr, found = rt.ResolveAddress(address)
	if !found {
		return address, fmt.Errorf("failed to resolve address %v to ID address even after sending zero balance", address)
	}

	return idAddr, nil
}

// Changed since v0:
// - Wrapping struct, added Penalty
type ApplyRewardParams struct {
	Reward  abi.TokenAmount
	Penalty abi.TokenAmount
}

// Discard is a helper
type Discard struct{}

func (d *Discard) MarshalCBOR(_ io.Writer) error {
	// serialization is a noop
	return nil
}

func (d *Discard) UnmarshalCBOR(_ io.Reader) error {
	// deserialization is a noop
	return nil
}
