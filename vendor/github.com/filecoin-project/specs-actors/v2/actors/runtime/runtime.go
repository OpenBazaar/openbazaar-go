package runtime

import (
	"context"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/go-state-types/rt"
	cid "github.com/ipfs/go-cid"

	"github.com/filecoin-project/specs-actors/v2/actors/runtime/proof"
)

// Interfaces for the runtime.
// These interfaces are not aliased onto prior versions even if they match exactly.
// Go's implicit interface satisfaction should mean that a single concrete type can satisfy
// many versions at the same time.

// Runtime is the interface to the execution environment for actor methods..
// This is everything that is accessible to actors, beyond parameters.
type Runtime interface {
	// Information related to the current message being executed.
	// When an actor invokes a method on another actor as a sub-call, these values reflect
	// the sub-call context, rather than the top-level context.
	Message

	// Provides a handle for the actor's state object.
	StateHandle

	// Provides IPLD storage for actor state
	Store

	// Provides the system call interface.
	Syscalls

	// The network protocol version number at the current epoch.
	NetworkVersion() network.Version

	// The current chain epoch number. The genesis block has epoch zero.
	CurrEpoch() abi.ChainEpoch

	// Satisfies the requirement that every exported actor method must invoke at least one caller validation
	// method before returning, without making any assertions about the caller.
	ValidateImmediateCallerAcceptAny()

	// Validates that the immediate caller's address exactly matches one of a set of expected addresses,
	// aborting if it does not.
	// The caller address is always normalized to an ID address, so expected addresses must be
	// ID addresses to have any expectation of passing validation.
	ValidateImmediateCallerIs(addrs ...addr.Address)

	// Validates that the immediate caller is an actor with code CID matching one of a set of
	// expected CIDs, aborting if it does not.
	ValidateImmediateCallerType(types ...cid.Cid)

	// The balance of the receiver. Always >= zero.
	CurrentBalance() abi.TokenAmount

	// Resolves an address of any protocol to an ID address (via the Init actor's table).
	// This allows resolution of externally-provided SECP, BLS, or actor addresses to the canonical form.
	// If the argument is an ID address it is returned directly.
	ResolveAddress(address addr.Address) (addr.Address, bool)

	// Look up the code ID at an actor address.
	// The address will be resolved as if via ResolveAddress, if necessary, so need not be an ID-address.
	GetActorCodeCID(addr addr.Address) (ret cid.Cid, ok bool)

	// GetRandomnessFromBeacon returns a (pseudo)random byte array drawing from a random beacon at a prior epoch.
	// The beacon value is combined with the personalization tag, epoch number, and explicitly provided entropy.
	// The personalization tag may be any int64 value.
	// The epoch must be less than the current epoch. The epoch may be negative, in which case
	// it addresses the beacon value from genesis block.
	// The entropy may be any byte array, or nil.
	GetRandomnessFromBeacon(personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) abi.Randomness

	// GetRandomnessFromTickets samples randomness from the ticket chain. Randomess
	// sampled through this method is unique per potential fork, and as a
	// result, processes relying on this randomness are tied to whichever fork
	// they choose.
	// See GetRandomnessFromBeacon for notes about the personalization tag, epoch, and entropy.
	GetRandomnessFromTickets(personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) abi.Randomness

	// Sends a message to another actor, returning the exit code and return value envelope.
	// If the invoked method does not return successfully, its state changes (and that of any messages it sent in turn)
	// will be rolled back.
	Send(toAddr addr.Address, methodNum abi.MethodNum, params cbor.Marshaler, value abi.TokenAmount, out cbor.Er) exitcode.ExitCode

	// Halts execution upon an error from which the receiver cannot recover. The caller will receive the exitcode and
	// an empty return value. State changes made within this call will be rolled back.
	// This method does not return.
	// The provided exit code must be >= exitcode.FirstActorExitCode.
	// The message and args are for diagnostic purposes and do not persist on chain. They should be suitable for
	// passing to fmt.Errorf(msg, args...).
	Abortf(errExitCode exitcode.ExitCode, msg string, args ...interface{})

	// Computes an address for a new actor. The returned address is intended to uniquely refer to
	// the actor even in the event of a chain re-org (whereas an ID-address might refer to a
	// different actor after messages are re-ordered).
	// Always an ActorExec address.
	NewActorAddress() addr.Address

	// Creates an actor with code `codeID` and address `address`, with empty state.
	// May only be called by Init actor.
	// Aborts if the provided address has previously been created.
	CreateActor(codeId cid.Cid, address addr.Address)

	// Deletes the executing actor from the state tree, transferring any balance to beneficiary.
	// Aborts if the beneficiary does not exist or is the calling actor.
	// May only be called by the actor itself.
	DeleteActor(beneficiary addr.Address)

	// Returns the total token supply in circulation at the beginning of the current epoch.
	// The circulating supply is the sum of:
	// - rewards emitted by the reward actor,
	// - funds vested from lock-ups in the genesis state,
	// less the sum of:
	// - funds burnt,
	// - pledge collateral locked in storage miner actors (recorded in the storage power actor)
	// - deal collateral locked by the storage market actor
	TotalFilCircSupply() abi.TokenAmount

	// Provides a Go context for use by HAMT, etc.
	// The VM is intended to provide an idealised machine abstraction, with infinite storage etc, so this context
	// should not be used by actor code directly.
	Context() context.Context

	// Starts a new tracing span. The span must be End()ed explicitly by invoking or deferring EndSpan
	StartSpan(name string) (EndSpan func())

	// ChargeGas charges specified amount of `gas` for execution.
	// `name` provides information about gas charging point
	// `virtual` sets virtual amount of gas to charge, this amount is not counted
	// toward execution cost. This functionality is used for observing global changes
	// in total gas charged if amount of gas charged was to be changed.
	ChargeGas(name string, gas int64, virtual int64)

	// Note events that may make debugging easier
	Log(level rt.LogLevel, msg string, args ...interface{})
}

// Store defines the storage module exposed to actors.
type Store interface {
	// Retrieves and deserializes an object from the store into `o`. Returns whether successful.
	StoreGet(c cid.Cid, o cbor.Unmarshaler) bool
	// Serializes and stores an object, returning its CID.
	StorePut(x cbor.Marshaler) cid.Cid
}

// Message contains information available to the actor about the executing message.
// These values are fixed for the duration of an invocation.
type Message interface {
	// The address of the immediate calling actor. Always an ID-address.
	// If an actor invokes its own method, Caller() == Receiver().
	Caller() addr.Address

	// The address of the actor receiving the message. Always an ID-address.
	Receiver() addr.Address

	// The value attached to the message being processed, implicitly added to CurrentBalance()
	// of Receiver() before method invocation.
	// This value came from Caller().
	ValueReceived() abi.TokenAmount
}

// Pure functions implemented as primitives by the runtime.
type Syscalls interface {
	// Verifies that a signature is valid for an address and plaintext.
	// If the address is a public-key type address, it is used directly.
	// If it's an ID-address, the actor is looked up in state. It must be an account actor, and the
	// public key is obtained from it's state.
	VerifySignature(signature crypto.Signature, signer addr.Address, plaintext []byte) error
	// Hashes input data using blake2b with 256 bit output.
	HashBlake2b(data []byte) [32]byte
	// Computes an unsealed sector CID (CommD) from its constituent piece CIDs (CommPs) and sizes.
	ComputeUnsealedSectorCID(reg abi.RegisteredSealProof, pieces []abi.PieceInfo) (cid.Cid, error)
	// Verifies a sector seal proof.
	VerifySeal(vi proof.SealVerifyInfo) error

	BatchVerifySeals(vis map[addr.Address][]proof.SealVerifyInfo) (map[addr.Address][]bool, error)

	// Verifies a proof of spacetime.
	VerifyPoSt(vi proof.WindowPoStVerifyInfo) error
	// Verifies that two block headers provide proof of a consensus fault:
	// - both headers mined by the same actor
	// - headers are different
	// - first header is of the same or lower epoch as the second
	// - the headers provide evidence of a fault (see the spec for the different fault types).
	// The parameters are all serialized block headers. The third "extra" parameter is consulted only for
	// the "parent grinding fault", in which case it must be the sibling of h1 (same parent tipset) and one of the
	// blocks in an ancestor of h2.
	// Returns nil and an error if the headers don't prove a fault.
	VerifyConsensusFault(h1, h2, extra []byte) (*ConsensusFault, error)
}

// StateHandle provides mutable, exclusive access to actor state.
type StateHandle interface {
	// Create initializes the state object.
	// This is only valid in a constructor function and when the state has not yet been initialized.
	StateCreate(obj cbor.Marshaler)

	// Readonly loads a readonly copy of the state into the argument.
	//
	// Any modification to the state is illegal and will result in an abort.
	StateReadonly(obj cbor.Unmarshaler)

	// Transaction loads a mutable version of the state into the `obj` argument and protects
	// the execution from side effects (including message send).
	//
	// The second argument is a function which allows the caller to mutate the state.
	//
	// If the state is modified after this function returns, execution will abort.
	//
	// The gas cost of this method is that of a Store.Put of the mutated state object.
	//
	// Note: the Go signature is not ideal due to lack of type system power.
	//
	// # Usage
	// ```go
	// var state SomeState
	// rt.StateTransaction(&state, func() {
	// 	// make some changes
	// 	state.ImLoaded = true
	// })
	// // state.ImLoaded = false // BAD!! state is readonly outside the lambda, it will panic
	// ```
	StateTransaction(obj cbor.Er, f func())
}
