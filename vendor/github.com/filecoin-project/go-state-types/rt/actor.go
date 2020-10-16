package rt

import (
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/ipfs/go-cid"
)

// VMActor is a concrete implementation of an actor, to be used by a Filecoin
// VM.
type VMActor interface {
	// Exports returns a slice of methods exported by this actor, indexed by
	// method number. Skipped/deprecated method numbers will be nil.
	Exports() []interface{}

	// Code returns the code ID for this actor.
	Code() cid.Cid

	// State returns a new State object for this actor. This can be used to
	// decode the actor's state.
	State() cbor.Er

	// NOTE: methods like "IsSingleton" are intentionally excluded from this
	// interface. That way, we can add additional attributes actors in newer
	// specs-actors versions, without having to update previous specs-actors
	// versions.
}

// IsSingletonActor returns true if the actor is a singleton actor (i.e., cannot
// be constructed).
func IsSingletonActor(a VMActor) bool {
	s, ok := a.(interface{ IsSingleton() bool })
	return ok && s.IsSingleton()
}
