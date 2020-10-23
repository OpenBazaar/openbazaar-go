package verifreg

import (
	addr "github.com/filecoin-project/go-address"
	abi "github.com/filecoin-project/go-state-types/abi"
	big "github.com/filecoin-project/go-state-types/big"
	cid "github.com/ipfs/go-cid"
)

// DataCap is an integer number of bytes.
// We can introduce policy changes and replace this in the future.
type DataCap = abi.StoragePower

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
