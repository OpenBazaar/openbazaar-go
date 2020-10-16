package market

import (
	"github.com/filecoin-project/go-state-types/abi"
	. "github.com/filecoin-project/specs-actors/v2/actors/util/adt"

	"github.com/ipfs/go-cid"
)

// A specialization of a array to deals.
// It is an error to query for a key that doesn't exist.
type DealArray struct {
	*Array
}

// Interprets a store as balance table with root `r`.
func AsDealProposalArray(s Store, r cid.Cid) (*DealArray, error) {
	a, err := AsArray(s, r)
	if err != nil {
		return nil, err
	}
	return &DealArray{a}, nil
}

// Returns the root cid of underlying AMT.
func (t *DealArray) Root() (cid.Cid, error) {
	return t.Array.Root()
}

// Gets the deal for a key. The entry must have been previously initialized.
func (t *DealArray) Get(id abi.DealID) (*DealProposal, bool, error) {
	var value DealProposal
	found, err := t.Array.Get(uint64(id), &value)
	return &value, found, err
}

func (t *DealArray) Set(k abi.DealID, value *DealProposal) error {
	return t.Array.Set(uint64(k), value)
}

func (t *DealArray) Delete(key uint64) error {
	return t.Array.Delete(key)
}

// A specialization of a array to deals.
// It is an error to query for a key that doesn't exist.
type DealMetaArray struct {
	*Array
}

type DealState struct {
	SectorStartEpoch abi.ChainEpoch // -1 if not yet included in proven sector
	LastUpdatedEpoch abi.ChainEpoch // -1 if deal state never updated
	SlashEpoch       abi.ChainEpoch // -1 if deal never slashed
}

// Interprets a store as balance table with root `r`.
func AsDealStateArray(s Store, r cid.Cid) (*DealMetaArray, error) {
	dsa, err := AsArray(s, r)
	if err != nil {
		return nil, err
	}

	return &DealMetaArray{dsa}, nil
}

// Returns the root cid of underlying AMT.
func (t *DealMetaArray) Root() (cid.Cid, error) {
	return t.Array.Root()
}

// Gets the deal for a key. The entry must have been previously initialized.
func (t *DealMetaArray) Get(id abi.DealID) (*DealState, bool, error) {
	var value DealState
	found, err := t.Array.Get(uint64(id), &value)
	if err != nil {
		return nil, false, err // The errors from Map carry good information, no need to wrap here.
	}
	if !found {
		return &DealState{
			SectorStartEpoch: epochUndefined,
			LastUpdatedEpoch: epochUndefined,
			SlashEpoch:       epochUndefined,
		}, false, nil
	}
	return &value, true, nil
}

func (t *DealMetaArray) Set(k abi.DealID, value *DealState) error {
	return t.Array.Set(uint64(k), value)
}

func (t *DealMetaArray) Delete(id abi.DealID) error {
	return t.Array.Delete(uint64(id))
}
