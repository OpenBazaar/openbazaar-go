package verifreg

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type StateSummary struct {
	Verifiers map[addr.Address]DataCap
	Clients   map[addr.Address]DataCap
}

// Checks internal invariants of verified registry state.
func CheckStateInvariants(st *State, store adt.Store) (*StateSummary, *builtin.MessageAccumulator, error) {
	acc := &builtin.MessageAccumulator{}
	acc.Require(st.RootKey.Protocol() == addr.ID, "root key %v should have ID protocol", st.RootKey)

	// Check verifiers
	verifiers, err := adt.AsMap(store, st.Verifiers)
	if err != nil {
		return nil, nil, err
	}

	allVerifiers := map[addr.Address]DataCap{}
	var vcap abi.StoragePower
	if err = verifiers.ForEach(&vcap, func(key string) error {
		verifier, err := addr.NewFromBytes([]byte(key))
		if err != nil {
			return err
		}
		acc.Require(verifier.Protocol() == addr.ID, "verifier %v should have ID protocol", verifier)
		acc.Require(vcap.GreaterThanEqual(big.Zero()), "verifier %v cap %v is negative", verifier, vcap)
		allVerifiers[verifier] = vcap.Copy()
		return nil
	}); err != nil {
		return nil, nil, err
	}

	// Check clients
	clients, err := adt.AsMap(store, st.VerifiedClients)
	if err != nil {
		return nil, nil, err
	}

	allClients := map[addr.Address]DataCap{}
	if err = clients.ForEach(&vcap, func(key string) error {
		client, err := addr.NewFromBytes([]byte(key))
		if err != nil {
			return err
		}
		acc.Require(client.Protocol() == addr.ID, "client %v should have ID protocol", client)
		acc.Require(vcap.GreaterThanEqual(big.Zero()), "client %v cap %v is negative", client, vcap)
		allClients[client] = vcap.Copy()
		return nil
	}); err != nil {
		return nil, nil, err
	}

	// Check verifiers and clients are disjoint.
	for v := range allVerifiers { //nolint:nomaprange
		_, found := allClients[v]
		acc.Require(!found, "verifier %v is also a client", v)
	}
	// No need to iterate all clients; any overlap must have been one of all verifiers.

	return &StateSummary{
		Verifiers: allVerifiers,
		Clients:   allClients,
	}, acc, nil
}
