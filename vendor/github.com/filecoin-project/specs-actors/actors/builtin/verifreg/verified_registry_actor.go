package verifreg

import (
	addr "github.com/filecoin-project/go-address"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	big "github.com/filecoin-project/specs-actors/actors/abi/big"
	builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	vmr "github.com/filecoin-project/specs-actors/actors/runtime"
	exitcode "github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	. "github.com/filecoin-project/specs-actors/actors/util"
	adt "github.com/filecoin-project/specs-actors/actors/util/adt"
)

type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.AddVerifier,
		3:                         a.RemoveVerifier,
		4:                         a.AddVerifiedClient,
		5:                         a.UseBytes,
		6:                         a.RestoreBytes,
	}
}

var _ abi.Invokee = Actor{}

////////////////////////////////////////////////////////////////////////////////
// Actor methods
////////////////////////////////////////////////////////////////////////////////

func (a Actor) Constructor(rt vmr.Runtime, rootKey *addr.Address) *adt.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to create verified registry state: %v", err)
	}

	st := ConstructState(emptyMap, *rootKey)
	rt.State().Create(st)
	return nil
}

type AddVerifierParams struct {
	Address   addr.Address
	Allowance DataCap
}

func (a Actor) AddVerifier(rt vmr.Runtime, params *AddVerifierParams) *adt.EmptyValue {
	var st State
	rt.State().Readonly(&st)
	rt.ValidateImmediateCallerIs(st.RootKey)

	rt.State().Transaction(&st, func() interface{} {
		err := st.PutVerifier(adt.AsStore(rt), params.Address, params.Allowance)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to add verifier: %v", err)
		}
		return nil
	})

	return nil
}

func (a Actor) RemoveVerifier(rt vmr.Runtime, verifierAddr *addr.Address) *adt.EmptyValue {
	var st State
	rt.State().Readonly(&st)
	rt.ValidateImmediateCallerIs(st.RootKey)

	rt.State().Transaction(&st, func() interface{} {
		err := st.DeleteVerifier(adt.AsStore(rt), *verifierAddr)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to delete verifier: %v", err)
		}
		return nil
	})

	return nil
}

type AddVerifiedClientParams struct {
	Address   addr.Address
	Allowance DataCap
}

func (a Actor) AddVerifiedClient(rt vmr.Runtime, params *AddVerifiedClientParams) *adt.EmptyValue {
	if params.Allowance.LessThanEqual(MinVerifiedDealSize) {
		rt.Abortf(exitcode.ErrIllegalArgument, "Allowance %d below MinVerifiedDealSize for add verified client %v", params.Allowance, params.Address)
	}
	rt.ValidateImmediateCallerAcceptAny()

	var st State
	rt.State().Transaction(&st, func() interface{} {
		// Validate caller is one of the verifiers.
		verifierAddr := rt.Message().Caller()
		verifierCap, found, err := st.GetVerifier(adt.AsStore(rt), verifierAddr)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "Failed to get Verifier for %v", err)
		} else if !found {
			rt.Abortf(exitcode.ErrNotFound, "Invalid verifier %v", verifierAddr)
		}

		// Compute new verifier cap and update.
		if verifierCap.LessThan(params.Allowance) {
			rt.Abortf(exitcode.ErrIllegalArgument, "Add more DataCap (%d) for VerifiedClient than allocated %d", params.Allowance, verifierCap)
		}
		newVerifierCap := big.Sub(*verifierCap, params.Allowance)

		if err := st.PutVerifier(adt.AsStore(rt), verifierAddr, newVerifierCap); err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "Failed to update new verifier cap (%d) for %v", newVerifierCap, verifierAddr)
		}

		// Write-once entry and does not get changed for simplicity.
		// If parties neeed more allowance, they can get another VerifiedClient account.
		// This is a one-time, upfront allocation.
		// Returns error if VerifiedClient already exists.
		_, found, err = st.GetVerifiedClient(adt.AsStore(rt), params.Address)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "Failed to load verified client state for %v", params.Address)
		} else if found {
			rt.Abortf(exitcode.ErrIllegalArgument, "Verified client already exists: %v", params.Address)
		}

		if err := st.PutVerifiedClient(adt.AsStore(rt), params.Address, params.Allowance); err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "Failed to add verified client %v with cap %d", params.Address, params.Allowance)
		}
		return nil
	})

	return nil
}

type UseBytesParams struct {
	Address  addr.Address     // Address of verified client.
	DealSize abi.StoragePower // Number of bytes to use.
}

// Called by StorageMarketActor during PublishStorageDeals.
// Do not allow partially verified deals (DealSize must be greater than equal to allowed cap).
// Delete VerifiedClient if remaining DataCap is smaller than minimum VerifiedDealSize.
func (a Actor) UseBytes(rt vmr.Runtime, params *UseBytesParams) *adt.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.StorageMarketActorAddr)

	if params.DealSize.LessThan(MinVerifiedDealSize) {
		rt.Abortf(exitcode.ErrIllegalArgument, "VerifiedDealSize: %d below minimum in UseBytes", params.DealSize)
	}

	var st State
	rt.State().Transaction(&st, func() interface{} {
		vcCap, found, err := st.GetVerifiedClient(adt.AsStore(rt), params.Address)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "Failed to get verified client state for %v", params.Address)
		} else if !found {
			rt.Abortf(exitcode.ErrIllegalArgument, "Invalid address for verified client %v", params.Address)
		}
		Assert(vcCap.GreaterThanEqual(big.Zero()))

		if params.DealSize.GreaterThan(vcCap) {
			rt.Abortf(exitcode.ErrIllegalArgument, "DealSize %d exceeds allowable cap: %d for VerifiedClient %v", params.DealSize, vcCap, params.Address)
		}

		newVcCap := big.Sub(vcCap, params.DealSize)
		if newVcCap.LessThan(MinVerifiedDealSize) {
			// Delete entry if remaining DataCap is less than MinVerifiedDealSize.
			// Will be restored later if the deal did not get activated with a ProvenSector.
			err = st.DeleteVerifiedClient(adt.AsStore(rt), params.Address)
			if err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "Failed to delete verified client %v with cap %d when %d bytes are used.", params.Address, vcCap, params.DealSize)
			}
		} else {
			err = st.PutVerifiedClient(adt.AsStore(rt), params.Address, newVcCap)
			if err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "Failed to update verified client %v when %d bytes are used.", params.Address, params.DealSize)
			}
		}

		return nil
	})

	return nil
}

type RestoreBytesParams struct {
	Address  addr.Address
	DealSize abi.StoragePower
}

// Called by HandleInitTimeoutDeals from StorageMarketActor when a VerifiedDeal fails to init.
// Restore allowable cap for the client, creating new entry if the client has been deleted.
func (a Actor) RestoreBytes(rt vmr.Runtime, params *RestoreBytesParams) *adt.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.StorageMarketActorAddr)

	if params.DealSize.LessThan(MinVerifiedDealSize) {
		rt.Abortf(exitcode.ErrIllegalArgument, "Below minimum VerifiedDealSize requested in RestoreBytes: %d", params.DealSize)
	}

	var st State
	rt.State().Transaction(&st, func() interface{} {
		vcCap, found, err := st.GetVerifiedClient(adt.AsStore(rt), params.Address)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "Failed to get verified client state for %v", params.Address)
		}

		if !found {
			vcCap = big.Zero()
		}

		newVcCap := big.Add(vcCap, params.DealSize)
		if err := st.PutVerifiedClient(adt.AsStore(rt), params.Address, newVcCap); err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "Failed to restore verified client state for %v", params.Address)
		}
		return nil
	})

	return nil
}
