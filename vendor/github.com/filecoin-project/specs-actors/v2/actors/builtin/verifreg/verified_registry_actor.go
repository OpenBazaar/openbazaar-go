package verifreg

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	verifreg0 "github.com/filecoin-project/specs-actors/actors/builtin/verifreg"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/runtime"
	. "github.com/filecoin-project/specs-actors/v2/actors/util"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
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

func (a Actor) Code() cid.Cid {
	return builtin.VerifiedRegistryActorCodeID
}

func (a Actor) IsSingleton() bool {
	return true
}

func (a Actor) State() cbor.Er {
	return new(State)
}

var _ runtime.VMActor = Actor{}

////////////////////////////////////////////////////////////////////////////////
// Actor methods
////////////////////////////////////////////////////////////////////////////////

func (a Actor) Constructor(rt runtime.Runtime, rootKey *addr.Address) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	// root should be an ID address
	idAddr, ok := rt.ResolveAddress(*rootKey)
	builtin.RequireParam(rt, ok, "root should be an ID address")

	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to create state")

	st := ConstructState(emptyMap, idAddr)
	rt.StateCreate(st)
	return nil
}

//type AddVerifierParams struct {
//	Address   addr.Address
//	Allowance DataCap
//}
type AddVerifierParams = verifreg0.AddVerifierParams

func (a Actor) AddVerifier(rt runtime.Runtime, params *AddVerifierParams) *abi.EmptyValue {
	if params.Allowance.LessThan(MinVerifiedDealSize) {
		rt.Abortf(exitcode.ErrIllegalArgument, "Allowance %d below MinVerifiedDealSize for add verifier %v", params.Allowance, params.Address)
	}

	verifier, err := builtin.ResolveToIDAddr(rt, params.Address)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to resolve verifier address %v to ID address", params.Address)

	var st State
	rt.StateReadonly(&st)
	rt.ValidateImmediateCallerIs(st.RootKey)

	if verifier == st.RootKey {
		rt.Abortf(exitcode.ErrIllegalArgument, "Rootkey cannot be added as verifier")
	}
	rt.StateTransaction(&st, func() {
		verifiers, err := adt.AsMap(adt.AsStore(rt), st.Verifiers)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load verifiers")

		verifiedClients, err := adt.AsMap(adt.AsStore(rt), st.VerifiedClients)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load verified clients")

		// A verified client cannot become a verifier
		found, err := verifiedClients.Get(abi.AddrKey(verifier), nil)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed get verified client state for %v", verifier)
		if found {
			rt.Abortf(exitcode.ErrIllegalArgument, "verified client %v cannot become a verifier", verifier)
		}

		err = verifiers.Put(abi.AddrKey(verifier), &params.Allowance)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add verifier")

		st.Verifiers, err = verifiers.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush verifiers")
	})

	return nil
}

func (a Actor) RemoveVerifier(rt runtime.Runtime, verifierAddr *addr.Address) *abi.EmptyValue {
	verifier, err := builtin.ResolveToIDAddr(rt, *verifierAddr)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to resolve verifier address %v to ID address", *verifierAddr)

	var st State
	rt.StateReadonly(&st)
	rt.ValidateImmediateCallerIs(st.RootKey)

	rt.StateTransaction(&st, func() {
		verifiers, err := adt.AsMap(adt.AsStore(rt), st.Verifiers)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load verifiers")

		err = verifiers.Delete(abi.AddrKey(verifier))
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to remove verifier")

		st.Verifiers, err = verifiers.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush verifiers")
	})

	return nil
}

//type AddVerifiedClientParams struct {
//	Address   addr.Address
//	Allowance DataCap
//}
type AddVerifiedClientParams = verifreg0.AddVerifiedClientParams

func (a Actor) AddVerifiedClient(rt runtime.Runtime, params *AddVerifiedClientParams) *abi.EmptyValue {
	// The caller will be verified by checking the verifiers table below.
	rt.ValidateImmediateCallerAcceptAny()

	if params.Allowance.LessThan(MinVerifiedDealSize) {
		rt.Abortf(exitcode.ErrIllegalArgument, "allowance %d below MinVerifiedDealSize for add verified client %v", params.Allowance, params.Address)
	}

	client, err := builtin.ResolveToIDAddr(rt, params.Address)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to resolve verified client address %v", params.Address)

	var st State
	rt.StateReadonly(&st)
	if st.RootKey == client {
		rt.Abortf(exitcode.ErrIllegalArgument, "Rootkey cannot be added as a verified client")
	}

	rt.StateTransaction(&st, func() {
		verifiers, err := adt.AsMap(adt.AsStore(rt), st.Verifiers)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load verifiers")

		verifiedClients, err := adt.AsMap(adt.AsStore(rt), st.VerifiedClients)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load verified clients")

		// Validate caller is one of the verifiers.
		verifier := rt.Caller()
		var verifierCap DataCap
		found, err := verifiers.Get(abi.AddrKey(verifier), &verifierCap)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get verifier %v", verifier)
		if !found {
			rt.Abortf(exitcode.ErrNotFound, "no such verifier %v", verifier)
		}

		// Validate client to be added isn't a verifier
		found, err = verifiers.Get(abi.AddrKey(client), nil)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get verifier")
		if found {
			rt.Abortf(exitcode.ErrIllegalArgument, "verifier %v cannot be added as a verified client", client)
		}

		// Compute new verifier cap and update.
		if verifierCap.LessThan(params.Allowance) {
			rt.Abortf(exitcode.ErrIllegalArgument, "add more DataCap (%d) for VerifiedClient than allocated %d", params.Allowance, verifierCap)
		}
		newVerifierCap := big.Sub(verifierCap, params.Allowance)

		err = verifiers.Put(abi.AddrKey(verifier), &newVerifierCap)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to update new verifier cap (%d) for %v", newVerifierCap, verifier)

		// This is a one-time, upfront allocation.
		// This allowance cannot be changed by calls to AddVerifiedClient as long as the client has not been removed.
		// If parties need more allowance, they need to create a new verified client or use up the the current allowance
		// and then create a new verified client.
		found, err = verifiedClients.Get(abi.AddrKey(client), &verifierCap)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get verified client %v", client)
		if found {
			rt.Abortf(exitcode.ErrIllegalArgument, "verified client already exists: %v", client)
		}

		err = verifiedClients.Put(abi.AddrKey(client), &params.Allowance)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to add verified client %v with cap %d", client, params.Allowance)

		st.Verifiers, err = verifiers.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush verifiers")

		st.VerifiedClients, err = verifiedClients.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush verified clients")
	})

	return nil
}

//type UseBytesParams struct {
//	Address  addr.Address     // Address of verified client.
//	DealSize abi.StoragePower // Number of bytes to use.
//}
type UseBytesParams = verifreg0.UseBytesParams

// Called by StorageMarketActor during PublishStorageDeals.
// Do not allow partially verified deals (DealSize must be greater than equal to allowed cap).
// Delete VerifiedClient if remaining DataCap is smaller than minimum VerifiedDealSize.
func (a Actor) UseBytes(rt runtime.Runtime, params *UseBytesParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.StorageMarketActorAddr)

	client, err := builtin.ResolveToIDAddr(rt, params.Address)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to resolve verified client address %v", params.Address)

	if params.DealSize.LessThan(MinVerifiedDealSize) {
		rt.Abortf(exitcode.ErrIllegalArgument, "VerifiedDealSize: %d below minimum in UseBytes", params.DealSize)
	}

	var st State
	rt.StateTransaction(&st, func() {
		verifiedClients, err := adt.AsMap(adt.AsStore(rt), st.VerifiedClients)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load verified clients")

		var vcCap DataCap
		found, err := verifiedClients.Get(abi.AddrKey(client), &vcCap)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get verified client %v", client)
		if !found {
			rt.Abortf(exitcode.ErrNotFound, "no such verified client %v", client)
		}
		Assert(vcCap.GreaterThanEqual(big.Zero()))

		if params.DealSize.GreaterThan(vcCap) {
			rt.Abortf(exitcode.ErrIllegalArgument, "DealSize %d exceeds allowable cap: %d for VerifiedClient %v", params.DealSize, vcCap, client)
		}

		newVcCap := big.Sub(vcCap, params.DealSize)
		if newVcCap.LessThan(MinVerifiedDealSize) {
			// Delete entry if remaining DataCap is less than MinVerifiedDealSize.
			// Will be restored later if the deal did not get activated with a ProvenSector.
			//
			// NOTE: Technically, client could lose up to MinVerifiedDealSize worth of DataCap.
			// See: https://github.com/filecoin-project/specs-actors/issues/727
			err = verifiedClients.Delete(abi.AddrKey(client))
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to delete verified client %v", client)
		} else {
			err = verifiedClients.Put(abi.AddrKey(client), &newVcCap)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to update verified client %v with %v", client, newVcCap)
		}

		st.VerifiedClients, err = verifiedClients.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush verified clients")
	})

	return nil
}

//type RestoreBytesParams struct {
//	Address  addr.Address
//	DealSize abi.StoragePower
//}
type RestoreBytesParams = verifreg0.RestoreBytesParams

// Called by HandleInitTimeoutDeals from StorageMarketActor when a VerifiedDeal fails to init.
// Restore allowable cap for the client, creating new entry if the client has been deleted.
func (a Actor) RestoreBytes(rt runtime.Runtime, params *RestoreBytesParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.StorageMarketActorAddr)

	if params.DealSize.LessThan(MinVerifiedDealSize) {
		rt.Abortf(exitcode.ErrIllegalArgument, "Below minimum VerifiedDealSize requested in RestoreBytes: %d", params.DealSize)
	}

	client, err := builtin.ResolveToIDAddr(rt, params.Address)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to resolve verified client addr %v", params.Address)

	var st State
	rt.StateReadonly(&st)
	if st.RootKey == client {
		rt.Abortf(exitcode.ErrIllegalArgument, "Cannot restore allowance for Rootkey")
	}

	rt.StateTransaction(&st, func() {
		verifiedClients, err := adt.AsMap(adt.AsStore(rt), st.VerifiedClients)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load verified clients")

		verifiers, err := adt.AsMap(adt.AsStore(rt), st.Verifiers)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load verifiers")

		// validate we are NOT attempting to do this for a verifier
		found, err := verifiers.Get(abi.AddrKey(client), nil)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed tp get verifier")
		if found {
			rt.Abortf(exitcode.ErrIllegalArgument, "cannot restore allowance for a verifier")
		}

		var vcCap DataCap
		found, err = verifiedClients.Get(abi.AddrKey(client), &vcCap)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get verified client %v", client)
		if !found {
			vcCap = big.Zero()
		}

		newVcCap := big.Add(vcCap, params.DealSize)
		err = verifiedClients.Put(abi.AddrKey(client), &newVcCap)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to put verified client %v with %v", client, newVcCap)

		st.VerifiedClients, err = verifiedClients.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load verifiers")
	})

	return nil
}
