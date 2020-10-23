package init

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	cid "github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	autil "github.com/filecoin-project/specs-actors/v2/actors/util"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type State struct {
	AddressMap  cid.Cid // HAMT[addr.Address]abi.ActorID
	NextID      abi.ActorID
	NetworkName string
}

func ConstructState(addressMapRoot cid.Cid, networkName string) *State {
	return &State{
		AddressMap:  addressMapRoot,
		NextID:      abi.ActorID(builtin.FirstNonSingletonActorId),
		NetworkName: networkName,
	}
}

// ResolveAddress resolves an address to an ID-address, if possible.
// If the provided address is an ID address, it is returned as-is.
// This means that mapped ID-addresses (which should only appear as values, not keys) and
// singleton actor addresses (which are not in the map) pass through unchanged.
//
// Returns an ID-address and `true` if the address was already an ID-address or was resolved in the mapping.
// Returns an undefined address and `false` if the address was not an ID-address and not found in the mapping.
// Returns an error only if state was inconsistent.
func (s *State) ResolveAddress(store adt.Store, address addr.Address) (addr.Address, bool, error) {
	// Short-circuit ID address resolution.
	if address.Protocol() == addr.ID {
		return address, true, nil
	}

	// Lookup address.
	m, err := adt.AsMap(store, s.AddressMap)
	if err != nil {
		return addr.Undef, false, xerrors.Errorf("failed to load address map: %w", err)
	}

	var actorID cbg.CborInt
	found, err := m.Get(abi.AddrKey(address), &actorID)
	if err != nil {
		return addr.Undef, false, xerrors.Errorf("failed to get from address map: %w", err)
	}
	if found {
		// Reconstruct address from the ActorID.
		idAddr, err2 := addr.NewIDAddress(uint64(actorID))
		autil.Assert(err2 == nil)
		return idAddr, true, nil
	} else {
		return addr.Undef, false, nil
	}
}

// Allocates a new ID address and stores a mapping of the argument address to it.
// Returns the newly-allocated address.
func (s *State) MapAddressToNewID(store adt.Store, address addr.Address) (addr.Address, error) {
	actorID := cbg.CborInt(s.NextID)
	s.NextID++

	m, err := adt.AsMap(store, s.AddressMap)
	if err != nil {
		return addr.Undef, xerrors.Errorf("failed to load address map: %w", err)
	}
	err = m.Put(abi.AddrKey(address), &actorID)
	if err != nil {
		return addr.Undef, xerrors.Errorf("map address failed to store entry: %w", err)
	}
	amr, err := m.Root()
	if err != nil {
		return addr.Undef, xerrors.Errorf("failed to get address map root: %w", err)
	}
	s.AddressMap = amr

	idAddr, err := addr.NewIDAddress(uint64(actorID))
	autil.Assert(err == nil)
	return idAddr, nil
}
