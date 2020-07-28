package mock

import (
	"context"
	"testing"

	addr "github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"github.com/minio/blake2b-simd"

	"github.com/filecoin-project/specs-actors/actors/abi"
)

// Build for fluent initialization of a mock runtime.
type RuntimeBuilder struct {
	rt *Runtime
}

// Initializes a new builder with a receiving actor address.
func NewBuilder(ctx context.Context, receiver addr.Address) *RuntimeBuilder {
	m := &Runtime{
		ctx:               ctx,
		epoch:             0,
		receiver:          receiver,
		caller:            addr.Address{},
		callerType:        cid.Undef,
		miner:             addr.Address{},
		idAddresses:       make(map[addr.Address]addr.Address),
		circulatingSupply: abi.NewTokenAmount(0),

		state:    cid.Undef,
		store:    make(map[cid.Cid][]byte),
		hashfunc: blake2b.Sum256,

		balance:       abi.NewTokenAmount(0),
		valueReceived: abi.NewTokenAmount(0),

		actorCodeCIDs: make(map[addr.Address]cid.Cid),
		newActorAddr:  addr.Undef,

		t:                        nil, // Initialized at Build()
		expectValidateCallerAny:  false,
		expectValidateCallerAddr: nil,
		expectValidateCallerType: nil,
		expectCreateActor:        nil,

		expectSends:      make([]*expectedMessage, 0),
		expectVerifySigs: make([]*expectVerifySig, 0),
	}
	return &RuntimeBuilder{m}
}

// Builds a new runtime object with the configured values.
func (b *RuntimeBuilder) Build(t testing.TB) *Runtime {
	cpy := *b.rt

	// Deep copy the mutable values.
	cpy.store = make(map[cid.Cid][]byte)
	for k, v := range b.rt.store {
		cpy.store[k] = v
	}

	cpy.t = t
	return &cpy
}

func (b *RuntimeBuilder) WithEpoch(epoch abi.ChainEpoch) *RuntimeBuilder {
	b.rt.epoch = epoch
	return b
}

func (b *RuntimeBuilder) WithCaller(address addr.Address, code cid.Cid) *RuntimeBuilder {
	b.rt.caller = address
	b.rt.callerType = code
	return b
}

func (b *RuntimeBuilder) WithMiner(miner addr.Address) *RuntimeBuilder {
	b.rt.miner = miner
	return b
}

func (b *RuntimeBuilder) WithBalance(balance, received abi.TokenAmount) *RuntimeBuilder {
	b.rt.balance = balance
	b.rt.valueReceived = received
	return b
}

func (b *RuntimeBuilder) WithActorType(addr addr.Address, code cid.Cid) *RuntimeBuilder {
	b.rt.actorCodeCIDs[addr] = code
	return b
}

func (b *RuntimeBuilder) WithHasher(f func(data []byte) [32]byte) *RuntimeBuilder {
	b.rt.hashfunc = f
	return b
}
