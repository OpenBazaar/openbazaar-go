package init

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type StateSummary struct {
	AddrIDs map[addr.Address]abi.ActorID
	NextID  abi.ActorID
}

// Checks internal invariants of init state.
func CheckStateInvariants(st *State, store adt.Store) (*StateSummary, *builtin.MessageAccumulator, error) {
	acc := &builtin.MessageAccumulator{}

	acc.Require(len(st.NetworkName) > 0, "network name is empty")
	acc.Require(st.NextID >= builtin.FirstNonSingletonActorId, "next id %d is too low", st.NextID)

	lut, err := adt.AsMap(store, st.AddressMap)
	if err != nil {
		return nil, nil, err
	}

	addrs := map[addr.Address]abi.ActorID{}
	reverse := map[abi.ActorID]addr.Address{}
	var value cbg.CborInt
	if err = lut.ForEach(&value, func(key string) error {
		actorId := abi.ActorID(value)
		keyAddr, err := addr.NewFromBytes([]byte(key))
		if err != nil {
			return err
		}

		acc.Require(keyAddr.Protocol() != addr.ID, "key %v is an ID address", keyAddr)
		acc.Require(keyAddr.Protocol() <= addr.BLS, "unknown address protocol for key %v", keyAddr)
		acc.Require(actorId >= builtin.FirstNonSingletonActorId, "unexpected singleton ID value %v", actorId)

		foundAddr, found := reverse[actorId]
		acc.Require(!found, "duplicate mapping to ID %v: %v, %v", actorId, keyAddr, foundAddr)
		reverse[actorId] = keyAddr

		addrs[keyAddr] = actorId
		return nil
	}); err != nil {
		return nil, nil, err
	}

	return &StateSummary{
		AddrIDs: addrs,
		NextID:  st.NextID,
	}, acc, nil
}
