package power_test

import (
	"bytes"
	"context"
	"strconv"
	"testing"

	addr "github.com/filecoin-project/go-address"
	cid "github.com/ipfs/go-cid"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	big "github.com/filecoin-project/specs-actors/actors/abi/big"
	builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	initact "github.com/filecoin-project/specs-actors/actors/builtin/init"
	power "github.com/filecoin-project/specs-actors/actors/builtin/power"
	vmr "github.com/filecoin-project/specs-actors/actors/runtime"
	exitcode "github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	adt "github.com/filecoin-project/specs-actors/actors/util/adt"
	mock "github.com/filecoin-project/specs-actors/support/mock"
	tutil "github.com/filecoin-project/specs-actors/support/testing"
)

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, power.Actor{})
}

func TestConstruction(t *testing.T) {
	actor := newHarness(t)
	owner := tutil.NewIDAddr(t, 101)
	miner := tutil.NewIDAddr(t, 103)
	actr := tutil.NewActorAddr(t, "actor")

	builder := mock.NewBuilder(context.Background(), builtin.StoragePowerActorAddr).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("simple construction", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)
	})

	t.Run("create miner", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		actor.createMiner(rt, owner, owner, miner, actr, abi.PeerID("miner"), []abi.Multiaddrs{{1}}, abi.RegisteredSealProof_StackedDrg2KiBV1, abi.NewTokenAmount(10))

		var st power.State
		rt.GetState(&st)
		assert.Equal(t, int64(1), st.MinerCount)
		assert.Equal(t, abi.NewStoragePower(0), st.TotalQualityAdjPower)
		assert.Equal(t, abi.NewStoragePower(0), st.TotalRawBytePower)
		assert.Equal(t, int64(0), st.NumMinersMeetingMinPower)

		claim, err := adt.AsMap(adt.AsStore(rt), st.Claims)
		assert.NoError(t, err)
		keys, err := claim.CollectKeys()
		require.NoError(t, err)
		assert.Equal(t, 1, len(keys))
		var actualClaim power.Claim
		found, err_ := claim.Get(asKey(keys[0]), &actualClaim)
		require.NoError(t, err_)
		assert.True(t, found)
		assert.Equal(t, power.Claim{big.Zero(), big.Zero()}, actualClaim) // miner has not proven anything

		verifyEmptyMap(t, rt, st.CronEventQueue)
	})
}

func TestPowerAndPledgeAccounting(t *testing.T) {
	actor := newHarness(t)
	owner := tutil.NewIDAddr(t, 101)
	miner1 := tutil.NewIDAddr(t, 111)
	miner2 := tutil.NewIDAddr(t, 112)

	// These tests use the min power for consensus to check the accounting above that value.
	// TODO: tests for crossing the consensus minimum boundary after settling the behaviour.
	// See https://github.com/filecoin-project/specs-actors/issues/266
	powerUnit := power.ConsensusMinerMinPower

	mul := func(a big.Int, b int64) big.Int {
		return big.Mul(a, big.NewInt(b))
	}

	builder := mock.NewBuilder(context.Background(), builtin.StoragePowerActorAddr).
		WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("power & pledge accounted", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		actor.createMinerBasic(rt, owner, owner, miner1)
		actor.createMinerBasic(rt, owner, owner, miner2)

		ret := actor.currentPowerTotal(rt)
		assert.Equal(t, big.Zero(), ret.RawBytePower)
		assert.Equal(t, big.Zero(), ret.QualityAdjPower)
		assert.Equal(t, big.Zero(), ret.PledgeCollateral)

		// Add power for miner1
		actor.updateClaimedPower(rt, miner1, powerUnit, mul(powerUnit, 2))
		ret = actor.currentPowerTotal(rt)
		assert.Equal(t, powerUnit, ret.RawBytePower)
		assert.Equal(t, mul(powerUnit, 2), ret.QualityAdjPower)
		assert.Equal(t, big.Zero(), ret.PledgeCollateral)

		// Add power and pledge for miner2
		actor.updateClaimedPower(rt, miner2, powerUnit, powerUnit)
		actor.updatePledgeTotal(rt, miner1, abi.NewTokenAmount(1e6))
		ret = actor.currentPowerTotal(rt)
		assert.Equal(t, mul(powerUnit, 2), ret.RawBytePower)
		assert.Equal(t, mul(powerUnit, 3), ret.QualityAdjPower)
		assert.Equal(t, abi.NewTokenAmount(1e6), ret.PledgeCollateral)

		rt.Verify()

		// Verify claims in state.
		var st power.State
		rt.GetState(&st)
		claim1, found, err := st.GetClaim(rt.AdtStore(), miner1)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, powerUnit, claim1.RawBytePower)
		require.Equal(t, mul(powerUnit, 2), claim1.QualityAdjPower)

		claim2, found, err := st.GetClaim(rt.AdtStore(), miner2)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, powerUnit, claim2.RawBytePower)
		require.Equal(t, powerUnit, claim2.QualityAdjPower)

		// Subtract power and some pledge for miner2
		actor.updateClaimedPower(rt, miner2, powerUnit.Neg(), powerUnit.Neg())
		actor.updatePledgeTotal(rt, miner2, abi.NewTokenAmount(1e5).Neg())
		ret = actor.currentPowerTotal(rt)
		assert.Equal(t, mul(powerUnit, 1), ret.RawBytePower)
		assert.Equal(t, mul(powerUnit, 2), ret.QualityAdjPower)
		assert.Equal(t, abi.NewTokenAmount(9e5), ret.PledgeCollateral)

		rt.GetState(&st)
		claim2, found, err = st.GetClaim(rt.AdtStore(), miner2)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, big.Zero(), claim2.RawBytePower)
		require.Equal(t, big.Zero(), claim2.QualityAdjPower)
	})
}

func TestCron(t *testing.T) {
	actor := newHarness(t)
	miner1 := tutil.NewIDAddr(t, 101)
	miner2 := tutil.NewIDAddr(t, 102)
	owner := tutil.NewIDAddr(t, 103)

	builder := mock.NewBuilder(context.Background(), builtin.StoragePowerActorAddr).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)

	t.Run("calls reward actor", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		expectedPower := big.NewInt(0)
		rt.SetEpoch(1)
		rt.ExpectValidateCallerAddr(builtin.CronActorAddr)
		rt.ExpectSend(builtin.RewardActorAddr, builtin.MethodsReward.UpdateNetworkKPI, &expectedPower, abi.NewTokenAmount(0), nil, 0)
		rt.SetCaller(builtin.CronActorAddr, builtin.CronActorCodeID)
		rt.Call(actor.Actor.OnEpochTickEnd, nil)
		rt.Verify()
	})

	t.Run("event scheduled in null round called next round", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		//  0 - genesis
		//  1 - block - registers events
		//  2 - null  - has event
		//  3 - null
		//  4 - block - has event

		rt.SetEpoch(1)
		actor.enrollCronEvent(rt, miner1, 2, []byte{0x1, 0x3})
		actor.enrollCronEvent(rt, miner2, 4, []byte{0x2, 0x3})

		expectedRawBytePower := big.NewInt(0)
		rt.SetEpoch(4)
		rt.ExpectValidateCallerAddr(builtin.CronActorAddr)
		rt.ExpectSend(miner1, builtin.MethodsMiner.OnDeferredCronEvent, vmr.CBORBytes([]byte{0x1, 0x3}), big.Zero(), nil, exitcode.Ok)
		rt.ExpectSend(miner2, builtin.MethodsMiner.OnDeferredCronEvent, vmr.CBORBytes([]byte{0x2, 0x3}), big.Zero(), nil, exitcode.Ok)
		rt.ExpectSend(builtin.RewardActorAddr, builtin.MethodsReward.UpdateNetworkKPI, &expectedRawBytePower, big.Zero(), nil, exitcode.Ok)
		rt.SetCaller(builtin.CronActorAddr, builtin.CronActorCodeID)
		rt.Call(actor.Actor.OnEpochTickEnd, nil)
		rt.Verify()
	})

	t.Run("handles failed call", func(t *testing.T) {
		rt := builder.Build(t)
		actor.constructAndVerify(rt)

		rt.SetEpoch(1)
		actor.enrollCronEvent(rt, miner1, 2, []byte{})
		actor.enrollCronEvent(rt, miner2, 2, []byte{})

		actor.createMinerBasic(rt, owner, owner, miner1)
		actor.createMinerBasic(rt, owner, owner, miner2)

		rawPow := power.ConsensusMinerMinPower
		qaPow := rawPow
		actor.updateClaimedPower(rt, miner1, rawPow, qaPow)
		startPow := actor.currentPowerTotal(rt)
		assert.Equal(t, rawPow, startPow.RawBytePower)
		assert.Equal(t, qaPow, startPow.QualityAdjPower)

		expectedPower := big.NewInt(0)
		rt.SetEpoch(2)
		rt.ExpectValidateCallerAddr(builtin.CronActorAddr)
		// First send fails
		rt.ExpectSend(miner1, builtin.MethodsMiner.OnDeferredCronEvent, vmr.CBORBytes([]byte{}), big.Zero(), nil, exitcode.ErrIllegalState)
		// Subsequent one still invoked
		rt.ExpectSend(miner2, builtin.MethodsMiner.OnDeferredCronEvent, vmr.CBORBytes([]byte{}), big.Zero(), nil, exitcode.Ok)
		// Reward actor still invoked
		rt.ExpectSend(builtin.RewardActorAddr, builtin.MethodsReward.UpdateNetworkKPI, &expectedPower, big.Zero(), nil, exitcode.Ok)
		rt.SetCaller(builtin.CronActorAddr, builtin.CronActorCodeID)
		rt.Call(actor.Actor.OnEpochTickEnd, nil)
		rt.Verify()

		// expect cron failure was logged
		rt.ExpectLogsContain("OnDeferredCronEvent failed for miner")

		newPow := actor.currentPowerTotal(rt)
		assert.Equal(t, abi.NewStoragePower(0), newPow.RawBytePower)
		assert.Equal(t, abi.NewStoragePower(0), newPow.QualityAdjPower)

		// Next epoch, only the reward actor is invoked
		rt.SetEpoch(3)
		rt.ExpectValidateCallerAddr(builtin.CronActorAddr)
		rt.ExpectSend(builtin.RewardActorAddr, builtin.MethodsReward.UpdateNetworkKPI, &expectedPower, big.Zero(), nil, exitcode.Ok)
		rt.SetCaller(builtin.CronActorAddr, builtin.CronActorCodeID)
		rt.Call(actor.Actor.OnEpochTickEnd, nil)
		rt.Verify()
	})
}

//
// Misc. Utility Functions
//

type key string

func asKey(in string) adt.Keyer {
	return key(in)
}

func verifyEmptyMap(t testing.TB, rt *mock.Runtime, cid cid.Cid) {
	mapChecked, err := adt.AsMap(adt.AsStore(rt), cid)
	assert.NoError(t, err)
	keys, err := mapChecked.CollectKeys()
	require.NoError(t, err)
	assert.Empty(t, keys)
}

type spActorHarness struct {
	power.Actor
	t        *testing.T
	minerSeq int
}

func newHarness(t *testing.T) *spActorHarness {
	return &spActorHarness{
		Actor: power.Actor{},
		t:     t,
	}
}

func (h *spActorHarness) constructAndVerify(rt *mock.Runtime) {
	rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
	ret := rt.Call(h.Actor.Constructor, nil)
	assert.Nil(h.t, ret)
	rt.Verify()

	var st power.State
	rt.GetState(&st)
	assert.Equal(h.t, abi.NewStoragePower(0), st.TotalRawBytePower)
	assert.Equal(h.t, abi.NewStoragePower(0), st.TotalQualityAdjPower)
	assert.Equal(h.t, abi.NewTokenAmount(0), st.TotalPledgeCollateral)
	assert.Equal(h.t, abi.ChainEpoch(-1), st.LastEpochTick)
	assert.Equal(h.t, int64(0), st.MinerCount)
	assert.Equal(h.t, int64(0), st.NumMinersMeetingMinPower)

	verifyEmptyMap(h.t, rt, st.Claims)
	verifyEmptyMap(h.t, rt, st.CronEventQueue)
}

func (h *spActorHarness) createMiner(rt *mock.Runtime, owner, worker, miner, robust addr.Address, peer abi.PeerID,
	multiaddrs []abi.Multiaddrs, sealProofType abi.RegisteredSealProof, value abi.TokenAmount) {
	createMinerParams := &power.CreateMinerParams{
		Owner:         owner,
		Worker:        worker,
		SealProofType: sealProofType,
		Peer:          peer,
		Multiaddrs:    multiaddrs,
	}

	// owner send CreateMiner to Actor
	rt.SetCaller(owner, builtin.AccountActorCodeID)
	rt.SetReceived(value)
	rt.SetBalance(value)
	rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)

	createMinerRet := &power.CreateMinerReturn{
		IDAddress:     miner,  // miner actor id address
		RobustAddress: robust, // should be long miner actor address
	}

	msgParams := &initact.ExecParams{
		CodeCID:           builtin.StorageMinerActorCodeID,
		ConstructorParams: initCreateMinerBytes(h.t, owner, worker, peer, multiaddrs, sealProofType),
	}
	rt.ExpectSend(builtin.InitActorAddr, builtin.MethodsInit.Exec, msgParams, value, createMinerRet, 0)
	rt.Call(h.Actor.CreateMiner, createMinerParams)
	rt.Verify()
}

func (h *spActorHarness) createMinerBasic(rt *mock.Runtime, owner, worker, miner addr.Address) {
	label := strconv.Itoa(h.minerSeq)
	actrAddr := tutil.NewActorAddr(h.t, label)
	h.minerSeq += 1
	h.createMiner(rt, owner, worker, miner, actrAddr, abi.PeerID(label), nil, abi.RegisteredSealProof_StackedDrg2KiBV1, big.Zero())
}

func (h *spActorHarness) updateClaimedPower(rt *mock.Runtime, miner addr.Address, rawDelta, qaDelta abi.StoragePower) {
	params := power.UpdateClaimedPowerParams{
		RawByteDelta:         rawDelta,
		QualityAdjustedDelta: qaDelta,
	}
	rt.SetCaller(miner, builtin.StorageMinerActorCodeID)
	rt.ExpectValidateCallerType(builtin.StorageMinerActorCodeID)
	rt.Call(h.UpdateClaimedPower, &params)
	rt.Verify()
}

func (h *spActorHarness) updatePledgeTotal(rt *mock.Runtime, miner addr.Address, delta abi.TokenAmount) {
	rt.SetCaller(miner, builtin.StorageMinerActorCodeID)
	rt.ExpectValidateCallerType(builtin.StorageMinerActorCodeID)
	rt.Call(h.UpdatePledgeTotal, &delta)
	rt.Verify()
}

func (h *spActorHarness) currentPowerTotal(rt *mock.Runtime) *power.CurrentTotalPowerReturn {
	rt.ExpectValidateCallerAny()
	ret := rt.Call(h.CurrentTotalPower, nil).(*power.CurrentTotalPowerReturn)
	rt.Verify()
	return ret
}

func (h *spActorHarness) enrollCronEvent(rt *mock.Runtime, miner addr.Address, epoch abi.ChainEpoch, payload []byte) {
	rt.ExpectValidateCallerType(builtin.StorageMinerActorCodeID)
	rt.SetCaller(miner, builtin.StorageMinerActorCodeID)
	rt.Call(h.Actor.EnrollCronEvent, &power.EnrollCronEventParams{
		EventEpoch: epoch,
		Payload:    payload,
	})
	rt.Verify()
}

func initCreateMinerBytes(t testing.TB, owner, worker addr.Address, peer abi.PeerID, multiaddrs []abi.Multiaddrs, sealProofType abi.RegisteredSealProof) []byte {
	params := &power.MinerConstructorParams{
		OwnerAddr:     owner,
		WorkerAddr:    worker,
		SealProofType: sealProofType,
		PeerId:        peer,
		Multiaddrs:    multiaddrs,
	}

	buf := new(bytes.Buffer)
	require.NoError(t, params.MarshalCBOR(buf))
	return buf.Bytes()
}

func (s key) Key() string {
	return string(s)
}
