package verifreg

import (
	addr "github.com/filecoin-project/go-address"
	cid "github.com/ipfs/go-cid"
	errors "github.com/pkg/errors"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	big "github.com/filecoin-project/specs-actors/actors/abi/big"
	adt "github.com/filecoin-project/specs-actors/actors/util/adt"
)

// DataCap is an integer number of bytes.
// We can introduce policy changes and replace this in the future.
type DataCap = abi.StoragePower
type AddrKey = adt.AddrKey

type State struct {
	// Root key holder multisig.
	// Authorize and remove verifiers.
	RootKey addr.Address

	// Verifiers authorize VerifiedClients.
	// Verifiers delegate their DataCap.
	Verifiers cid.Cid // HAMT[addr.Address]DataCap

	// VerifiedClients can add VerifiedClientData, up to DataCap.
	VerifiedClients cid.Cid // HAMT[addr.Address]DataCap
}

var MinVerifiedDealSize abi.StoragePower = big.NewInt(1 << 20) // PARAM_FINISH

// rootKeyAddress comes from genesis.
func ConstructState(emptyMapCid cid.Cid, rootKeyAddress addr.Address) *State {
	return &State{
		RootKey:         rootKeyAddress,
		Verifiers:       emptyMapCid,
		VerifiedClients: emptyMapCid,
	}
}

func (st *State) PutVerifier(store adt.Store, verifierAddr addr.Address, verifierCap DataCap) error {
	verifiers, err := adt.AsMap(store, st.Verifiers)
	if err != nil {
		return err
	}

	if err := verifiers.Put(AddrKey(verifierAddr), &verifierCap); err != nil {
		return errors.Wrapf(err, "failed to put verifier %v with a cap of %v", verifierAddr, verifierCap)
	}
	st.Verifiers, err = verifiers.Root()
	if err != nil {
		return errors.Wrapf(err, "failed to flush Verifiers in PutVerifier")
	}
	return nil
}

func (st *State) GetVerifier(store adt.Store, address addr.Address) (*DataCap, bool, error) {
	verifiers, err := adt.AsMap(store, st.Verifiers)
	if err != nil {
		return nil, false, err
	}

	var allowance DataCap
	found, err := verifiers.Get(AddrKey(address), &allowance)
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to load verifier for address %v", address)
	}
	return &allowance, found, nil
}

func (st *State) DeleteVerifier(store adt.Store, address addr.Address) error {
	verifiers, err := adt.AsMap(store, st.Verifiers)
	if err != nil {
		return err
	}

	if err := verifiers.Delete(AddrKey(address)); err != nil {
		return errors.Wrapf(err, "failed to delete verifier for address %v", address)
	}
	st.Verifiers, err = verifiers.Root()
	if err != nil {
		return errors.Wrapf(err, "failed to flush Verifiers in DeleteVerifier")
	}
	return nil
}

func (st *State) PutVerifiedClient(store adt.Store, vcAddress addr.Address, vcCap DataCap) error {
	vc, err := adt.AsMap(store, st.VerifiedClients)
	if err != nil {
		return err
	}

	if err := vc.Put(AddrKey(vcAddress), &vcCap); err != nil {
		return err
	}
	st.VerifiedClients, err = vc.Root()
	if err != nil {
		return errors.Wrapf(err, "failed to flush VerifiedClients in PutVerifiedClient")
	}
	return nil
}

func (st *State) GetVerifiedClient(store adt.Store, vcAddress addr.Address) (DataCap, bool, error) {
	vc, err := adt.AsMap(store, st.VerifiedClients)
	if err != nil {
		return big.Zero(), false, err
	}

	var allowance DataCap
	found, err := vc.Get(AddrKey(vcAddress), &allowance)
	if err != nil {
		return big.Zero(), false, errors.Wrapf(err, "failed to load verified client for address %v", vcAddress)
	}
	return allowance, found, nil
}

func (st *State) DeleteVerifiedClient(store adt.Store, vcAddress addr.Address) error {
	vc, err := adt.AsMap(store, st.VerifiedClients)
	if err != nil {
		return err
	}

	if err := vc.Delete(AddrKey(vcAddress)); err != nil {
		return errors.Wrapf(err, "failed to delete verified client for address %v", vcAddress)
	}
	st.VerifiedClients, err = vc.Root()
	if err != nil {
		return errors.Wrapf(err, "failed to flush VerifiedClients in DeleteVerifiedClient")
	}
	return nil
}
