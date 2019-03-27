package obj

import (
	"reflect"

	. "gx/ipfs/QmdBzoMxsBpojBfN1cv5GnKtB7sfYBMoLH7p9qSyEVYXcu/refmt/tok"
)

type unmarshalMachineArrayWildcard struct {
	target_rv reflect.Value
	value_rt  reflect.Type
	valueMach UnmarshalMachine
	step      unmarshalMachineStep
	index     int
	maxLen    int
}

func (mach *unmarshalMachineArrayWildcard) Reset(slab *unmarshalSlab, rv reflect.Value, rt reflect.Type) error {
	mach.target_rv = rv
	mach.value_rt = rt.Elem()
	mach.valueMach = slab.requisitionMachine(mach.value_rt)
	mach.step = mach.step_Initial
	mach.index = 0
	mach.maxLen = rt.Len()
	return nil
}

func (mach *unmarshalMachineArrayWildcard) Step(driver *Unmarshaller, slab *unmarshalSlab, tok *Token) (done bool, err error) {
	return mach.step(driver, slab, tok)
}

func (mach *unmarshalMachineArrayWildcard) step_Initial(_ *Unmarshaller, slab *unmarshalSlab, tok *Token) (done bool, err error) {
	// If it's a special state, start an object.
	//  (Or, blow up if its a special state that's silly).
	switch tok.Type {
	case TMapOpen:
		return true, ErrMalformedTokenStream{tok.Type, "start of array"}
	case TArrOpen:
		// Great.  Consumed.
		mach.step = mach.step_AcceptValue
		// Initialize the array.  Its length is encoded in its type.
		mach.target_rv.Set(reflect.Zero(mach.target_rv.Type()))
		return false, nil
	case TMapClose:
		return true, ErrMalformedTokenStream{tok.Type, "start of array"}
	case TArrClose:
		return true, ErrMalformedTokenStream{tok.Type, "start of array"}
	case TNull:
		mach.target_rv.Set(reflect.Zero(mach.target_rv.Type()))
		return true, nil
	default:
		return true, ErrMalformedTokenStream{tok.Type, "start of array"}
	}
}

func (mach *unmarshalMachineArrayWildcard) step_AcceptValue(driver *Unmarshaller, slab *unmarshalSlab, tok *Token) (done bool, err error) {
	// Either form of open token are valid, but
	// - an arrClose is ours
	// - and a mapClose is clearly invalid.
	switch tok.Type {
	case TMapClose:
		// no special checks for ends of wildcard slice; no such thing as incomplete.
		return true, ErrMalformedTokenStream{tok.Type, "start of value or end of array"}
	case TArrClose:
		// release the slab row we requisitioned for our value machine.
		slab.release()
		return true, nil
	}

	// Return an error if we're about to exceed our length limit.
	if mach.index >= mach.maxLen {
		return true, ErrMalformedTokenStream{tok.Type, "end of array (out of space)"}
	}

	// Recurse on a handle to the next index.
	rv := mach.target_rv.Index(mach.index)
	mach.index++
	return false, driver.Recurse(tok, rv, mach.value_rt, mach.valueMach)
	// Step simply remains `step_AcceptValue` -- arrays don't have much state machine.
}
