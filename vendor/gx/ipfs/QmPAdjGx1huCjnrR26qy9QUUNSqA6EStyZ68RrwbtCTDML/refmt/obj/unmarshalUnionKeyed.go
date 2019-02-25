package obj

import (
	"reflect"

	"gx/ipfs/QmPAdjGx1huCjnrR26qy9QUUNSqA6EStyZ68RrwbtCTDML/refmt/obj/atlas"
	. "gx/ipfs/QmPAdjGx1huCjnrR26qy9QUUNSqA6EStyZ68RrwbtCTDML/refmt/tok"
)

type unmarshalMachineUnionKeyed struct {
	cfg *atlas.UnionKeyedMorphism // set on initialization

	target_rv reflect.Value
	target_rt reflect.Type

	step     unmarshalMachineStep
	tmp_rv   reflect.Value
	delegate UnmarshalMachine // actual machine, once we've demuxed with the second token (the key).
}

func (mach *unmarshalMachineUnionKeyed) Reset(_ *unmarshalSlab, rv reflect.Value, rt reflect.Type) error {
	mach.target_rv = rv
	mach.target_rt = rt
	mach.step = mach.acceptMapOpen
	return nil
}

func (mach *unmarshalMachineUnionKeyed) Step(driver *Unmarshaller, slab *unmarshalSlab, tok *Token) (done bool, err error) {
	return mach.step(driver, slab, tok)
}

func (mach *unmarshalMachineUnionKeyed) acceptMapOpen(driver *Unmarshaller, slab *unmarshalSlab, tok *Token) (done bool, err error) {
	switch tok.Type {
	case TMapOpen:
		switch tok.Length {
		case -1: // pass
		case 1: // correct
		default:
			return true, ErrMalformedTokenStream{tok.Type, "unions in keyed format must be maps with exactly one entry"} // FIXME not malformed per se
		}
		mach.step = mach.acceptKey
		return false, nil
	// REVIEW: is case TNull perhaps conditionally acceptable?
	default:
		return true, ErrMalformedTokenStream{tok.Type, "start of union value"} // FIXME not malformed per se
	}
}

func (mach *unmarshalMachineUnionKeyed) acceptKey(driver *Unmarshaller, slab *unmarshalSlab, tok *Token) (done bool, err error) {
	switch tok.Type {
	case TString:
		// Look up the configuration for this key.
		delegateAtlasEnt, ok := mach.cfg.Elements[tok.Str]
		if !ok {
			return true, ErrNoSuchUnionMember{tok.Str, mach.target_rt, mach.cfg.KnownMembers}
		}
		// Allocate a new concrete value, and hang on to that rv handle.
		//  Assigning into the interface must be done at the end if it's a non-pointer.
		mach.tmp_rv = reflect.New(delegateAtlasEnt.Type).Elem()
		// Get and configure a machine for the delegation.
		delegate := _yieldUnmarshalMachinePtrForAtlasEntry(slab.tip(), delegateAtlasEnt, slab.atlas)
		if err := delegate.Reset(slab, mach.tmp_rv, delegateAtlasEnt.Type); err != nil {
			return true, err
		}
		mach.delegate = delegate
		mach.step = mach.doDelegate
		return false, nil
	default:
		return true, ErrMalformedTokenStream{tok.Type, "map key"}
	}
}

func (mach *unmarshalMachineUnionKeyed) doDelegate(driver *Unmarshaller, slab *unmarshalSlab, tok *Token) (done bool, err error) {
	done, err = mach.delegate.Step(driver, slab, tok)
	if done && err == nil {
		mach.step = mach.acceptMapClose
		return false, nil
	}
	return
}

func (mach *unmarshalMachineUnionKeyed) acceptMapClose(driver *Unmarshaller, slab *unmarshalSlab, tok *Token) (done bool, err error) {
	switch tok.Type {
	case TMapClose:
		mach.target_rv.Set(mach.tmp_rv)
		return true, nil
	default:
		return true, ErrMalformedTokenStream{tok.Type, "map close at end of union value"}
	}
}
