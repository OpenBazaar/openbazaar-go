package power

import (
	"bytes"

	"github.com/filecoin-project/go-address"
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	rtt "github.com/filecoin-project/go-state-types/rt"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/specs-actors/actors/builtin"
	initact "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/proof"
	. "github.com/filecoin-project/specs-actors/actors/util"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/filecoin-project/specs-actors/actors/util/smoothing"
)

type Runtime = runtime.Runtime

type SectorTermination int64

const (
	ErrTooManyProveCommits = exitcode.FirstActorSpecificExitCode + iota
)

type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.CreateMiner,
		3:                         a.UpdateClaimedPower,
		4:                         a.EnrollCronEvent,
		5:                         a.OnEpochTickEnd,
		6:                         a.UpdatePledgeTotal,
		7:                         a.OnConsensusFault,
		8:                         a.SubmitPoRepForBulkVerify,
		9:                         a.CurrentTotalPower,
	}
}

func (a Actor) Code() cid.Cid {
	return builtin.StoragePowerActorCodeID
}

func (a Actor) IsSingleton() bool {
	return true
}

func (a Actor) State() cbor.Er {
	return new(State)
}

var _ runtime.VMActor = Actor{}

// Storage miner actor constructor params are defined here so the power actor can send them to the init actor
// to instantiate miners.
type MinerConstructorParams struct {
	OwnerAddr     addr.Address
	WorkerAddr    addr.Address
	ControlAddrs  []addr.Address
	SealProofType abi.RegisteredSealProof
	PeerId        abi.PeerID
	Multiaddrs    []abi.Multiaddrs
}

type SectorStorageWeightDesc struct {
	SectorSize         abi.SectorSize
	Duration           abi.ChainEpoch
	DealWeight         abi.DealWeight
	VerifiedDealWeight abi.DealWeight
}

////////////////////////////////////////////////////////////////////////////////
// Actor methods
////////////////////////////////////////////////////////////////////////////////

func (a Actor) Constructor(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")
	emptyMMapCid, err := adt.MakeEmptyMultimap(adt.AsStore(rt)).Root()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to construct state")

	st := ConstructState(emptyMap, emptyMMapCid)
	rt.StateCreate(st)
	return nil
}

type CreateMinerParams struct {
	Owner         addr.Address
	Worker        addr.Address
	SealProofType abi.RegisteredSealProof
	Peer          abi.PeerID
	Multiaddrs    []abi.Multiaddrs
}

type CreateMinerReturn struct {
	IDAddress     addr.Address // The canonical ID-based address for the actor.
	RobustAddress addr.Address // A more expensive but re-org-safe address for the newly created actor.
}

func (a Actor) CreateMiner(rt Runtime, params *CreateMinerParams) *CreateMinerReturn {
	rt.ValidateImmediateCallerType(builtin.CallerTypesSignable...)

	ctorParams := MinerConstructorParams{
		OwnerAddr:     params.Owner,
		WorkerAddr:    params.Worker,
		SealProofType: params.SealProofType,
		PeerId:        params.Peer,
		Multiaddrs:    params.Multiaddrs,
	}
	ctorParamBuf := new(bytes.Buffer)
	err := ctorParams.MarshalCBOR(ctorParamBuf)
	builtin.RequireNoErr(rt, err, exitcode.ErrSerialization, "failed to serialize miner constructor params %v", ctorParams)

	var addresses initact.ExecReturn
	code := rt.Send(
		builtin.InitActorAddr,
		builtin.MethodsInit.Exec,
		&initact.ExecParams{
			CodeCID:           builtin.StorageMinerActorCodeID,
			ConstructorParams: ctorParamBuf.Bytes(),
		},
		rt.ValueReceived(), // Pass on any value to the new actor.
		&addresses,
	)
	builtin.RequireSuccess(rt, code, "failed to init new actor")

	var st State
	rt.StateTransaction(&st, func() {
		claims, err := adt.AsMap(adt.AsStore(rt), st.Claims)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load claims")

		err = setClaim(claims, addresses.IDAddress, &Claim{abi.NewStoragePower(0), abi.NewStoragePower(0)})
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to put power in claimed table while creating miner")

		st.MinerCount += 1

		st.Claims, err = claims.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush claims")
	})
	return &CreateMinerReturn{
		IDAddress:     addresses.IDAddress,
		RobustAddress: addresses.RobustAddress,
	}
}

type UpdateClaimedPowerParams struct {
	RawByteDelta         abi.StoragePower
	QualityAdjustedDelta abi.StoragePower
}

// Adds or removes claimed power for the calling actor.
// May only be invoked by a miner actor.
func (a Actor) UpdateClaimedPower(rt Runtime, params *UpdateClaimedPowerParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)
	minerAddr := rt.Caller()
	var st State
	rt.StateTransaction(&st, func() {
		claims, err := adt.AsMap(adt.AsStore(rt), st.Claims)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load claims")

		err = st.addToClaim(claims, minerAddr, params.RawByteDelta, params.QualityAdjustedDelta)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to update power raw %s, qa %s", params.RawByteDelta, params.QualityAdjustedDelta)

		st.Claims, err = claims.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush claims")
	})
	return nil
}

type EnrollCronEventParams struct {
	EventEpoch abi.ChainEpoch
	Payload    []byte
}

func (a Actor) EnrollCronEvent(rt Runtime, params *EnrollCronEventParams) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)
	minerAddr := rt.Caller()
	minerEvent := CronEvent{
		MinerAddr:       minerAddr,
		CallbackPayload: params.Payload,
	}

	// Ensure it is not possible to enter a large negative number which would cause problems in cron processing.
	if params.EventEpoch < 0 {
		rt.Abortf(exitcode.ErrIllegalArgument, "cron event epoch %d cannot be less than zero", params.EventEpoch)
	}

	var st State
	rt.StateTransaction(&st, func() {
		events, err := adt.AsMultimap(adt.AsStore(rt), st.CronEventQueue)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load cron events")

		err = st.appendCronEvent(events, params.EventEpoch, &minerEvent)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to enroll cron event")

		st.CronEventQueue, err = events.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush cron events")
	})
	return nil
}

// Called by Cron.
func (a Actor) OnEpochTickEnd(rt Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.CronActorAddr)

	a.processDeferredCronEvents(rt)
	a.processBatchProofVerifies(rt)

	var st State
	rt.StateTransaction(&st, func() {
		// update next epoch's power and pledge values
		// this must come before the next epoch's rewards are calculated
		// so that next epoch reward reflects power added this epoch
		rawBytePower, qaPower := CurrentTotalPower(&st)
		st.ThisEpochPledgeCollateral = st.TotalPledgeCollateral
		st.ThisEpochQualityAdjPower = qaPower
		st.ThisEpochRawBytePower = rawBytePower
		delta := rt.CurrEpoch() - st.LastProcessedCronEpoch
		st.updateSmoothedEstimate(delta)

		st.LastProcessedCronEpoch = rt.CurrEpoch()
	})

	// update network KPI in RewardActor
	code := rt.Send(
		builtin.RewardActorAddr,
		builtin.MethodsReward.UpdateNetworkKPI,
		&st.ThisEpochRawBytePower,
		abi.NewTokenAmount(0),
		&builtin.Discard{},
	)
	builtin.RequireSuccess(rt, code, "failed to update network KPI with Reward Actor")

	return nil
}

func (a Actor) UpdatePledgeTotal(rt Runtime, pledgeDelta *abi.TokenAmount) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)
	var st State
	rt.StateTransaction(&st, func() {
		st.addPledgeTotal(*pledgeDelta)
	})
	return nil
}

func (a Actor) OnConsensusFault(rt Runtime, pledgeAmount *abi.TokenAmount) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)
	minerAddr := rt.Caller()

	var st State
	rt.StateTransaction(&st, func() {
		claims, err := adt.AsMap(adt.AsStore(rt), st.Claims)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load claims")

		claim, powerOk, err := getClaim(claims, minerAddr)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to read claimed power for fault")
		if !powerOk {
			rt.Abortf(exitcode.ErrNotFound, "miner %v not registered (already slashed?)", minerAddr)
		}
		Assert(claim.RawBytePower.GreaterThanEqual(big.Zero()))
		Assert(claim.QualityAdjPower.GreaterThanEqual(big.Zero()))
		err = st.addToClaim(claims, minerAddr, claim.RawBytePower.Neg(), claim.QualityAdjPower.Neg())
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "could not add to claim for %s after loading existing claim for this address", minerAddr)

		st.addPledgeTotal(pledgeAmount.Neg())

		// delete miner actor claims
		err = claims.Delete(abi.AddrKey(minerAddr))
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to remove miner %v", minerAddr)

		st.MinerCount -= 1

		st.Claims, err = claims.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush claims")
	})

	return nil
}

// GasOnSubmitVerifySeal is amount of gas charged for SubmitPoRepForBulkVerify
// This number is empirically determined
const GasOnSubmitVerifySeal = 34721049

func (a Actor) SubmitPoRepForBulkVerify(rt Runtime, sealInfo *proof.SealVerifyInfo) *abi.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)

	minerAddr := rt.Caller()

	var st State
	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)
		var mmap *adt.Multimap
		if st.ProofValidationBatch == nil {
			mmap = adt.MakeEmptyMultimap(store)
		} else {
			var err error
			mmap, err = adt.AsMultimap(adt.AsStore(rt), *st.ProofValidationBatch)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load proof batch set")
		}

		arr, found, err := mmap.Get(abi.AddrKey(minerAddr))
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to get get seal verify infos at addr %s", minerAddr)
		if found && arr.Length() >= MaxMinerProveCommitsPerEpoch {
			rt.Abortf(ErrTooManyProveCommits, "miner %s attempting to prove commit over %d sectors in epoch", minerAddr, MaxMinerProveCommitsPerEpoch)
		}

		err = mmap.Add(abi.AddrKey(minerAddr), sealInfo)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to insert proof into batch")

		mmrc, err := mmap.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush proof batch")

		rt.ChargeGas("OnSubmitVerifySeal", GasOnSubmitVerifySeal, 0)
		st.ProofValidationBatch = &mmrc
	})

	return nil
}

type CurrentTotalPowerReturn struct {
	RawBytePower            abi.StoragePower
	QualityAdjPower         abi.StoragePower
	PledgeCollateral        abi.TokenAmount
	QualityAdjPowerSmoothed *smoothing.FilterEstimate
}

// Returns the total power and pledge recorded by the power actor.
// The returned values are frozen during the cron tick before this epoch
// so that this method returns consistent values while processing all messages
// of an epoch.
func (a Actor) CurrentTotalPower(rt Runtime, _ *abi.EmptyValue) *CurrentTotalPowerReturn {
	rt.ValidateImmediateCallerAcceptAny()
	var st State
	rt.StateReadonly(&st)

	return &CurrentTotalPowerReturn{
		RawBytePower:            st.ThisEpochRawBytePower,
		QualityAdjPower:         st.ThisEpochQualityAdjPower,
		PledgeCollateral:        st.ThisEpochPledgeCollateral,
		QualityAdjPowerSmoothed: st.ThisEpochQAPowerSmoothed,
	}
}

////////////////////////////////////////////////////////////////////////////////
// Method utility functions
////////////////////////////////////////////////////////////////////////////////

func (a Actor) processBatchProofVerifies(rt Runtime) {
	var st State

	var miners []address.Address
	verifies := make(map[address.Address][]proof.SealVerifyInfo)

	rt.StateTransaction(&st, func() {
		store := adt.AsStore(rt)
		if st.ProofValidationBatch == nil {
			return
		}
		mmap, err := adt.AsMultimap(store, *st.ProofValidationBatch)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load proofs validation batch")

		err = mmap.ForAll(func(k string, arr *adt.Array) error {
			a, err := address.NewFromBytes([]byte(k))
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to parse address key")

			miners = append(miners, a)

			var infos []proof.SealVerifyInfo
			var svi proof.SealVerifyInfo
			err = arr.ForEach(&svi, func(i int64) error {
				infos = append(infos, svi)
				return nil
			})
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to iterate over proof verify array for miner %s", a)

			verifies[a] = infos
			return nil
		})
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to iterate proof batch")

		st.ProofValidationBatch = nil
	})

	res, err := rt.BatchVerifySeals(verifies)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to batch verify")

	for _, m := range miners {
		vres, ok := res[m]
		if !ok {
			rt.Abortf(exitcode.ErrNotFound, "batch verify seals syscall implemented incorrectly")
		}

		verifs := verifies[m]

		seen := map[abi.SectorNumber]struct{}{}
		var successful []abi.SectorNumber
		for i, r := range vres {
			if r {
				snum := verifs[i].SectorID.Number

				if _, exists := seen[snum]; exists {
					// filter-out duplicates
					continue
				}

				seen[snum] = struct{}{}
				successful = append(successful, snum)
			}
		}

		// The exit code is explicitly ignored
		_ = rt.Send(
			m,
			builtin.MethodsMiner.ConfirmSectorProofsValid,
			&builtin.ConfirmSectorProofsParams{Sectors: successful},
			abi.NewTokenAmount(0),
			&builtin.Discard{},
		)
	}
}

func (a Actor) processDeferredCronEvents(rt Runtime) {
	rtEpoch := rt.CurrEpoch()

	var cronEvents []CronEvent
	var st State
	rt.StateTransaction(&st, func() {
		events, err := adt.AsMultimap(adt.AsStore(rt), st.CronEventQueue)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load cron events")

		for epoch := st.FirstCronEpoch; epoch <= rtEpoch; epoch++ {
			epochEvents, err := loadCronEvents(events, epoch)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load cron events at %v", epoch)

			cronEvents = append(cronEvents, epochEvents...)

			if len(epochEvents) > 0 {
				err = events.RemoveAll(epochKey(epoch))
				builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to clear cron events at %v", epoch)
			}
		}

		st.FirstCronEpoch = rtEpoch + 1

		st.CronEventQueue, err = events.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush events")
	})
	failedMinerCrons := make([]addr.Address, 0)
	for _, event := range cronEvents {
		code := rt.Send(
			event.MinerAddr,
			builtin.MethodsMiner.OnDeferredCronEvent,
			runtime.CBORBytes(event.CallbackPayload),
			abi.NewTokenAmount(0),
			&builtin.Discard{},
		)
		// If a callback fails, this actor continues to invoke other callbacks
		// and persists state removing the failed event from the event queue. It won't be tried again.
		// Failures are unexpected here but will result in removal of miner power
		// A log message would really help here.
		if code != exitcode.Ok {
			rt.Log(rtt.WARN, "OnDeferredCronEvent failed for miner %s: exitcode %d", event.MinerAddr, code)
			failedMinerCrons = append(failedMinerCrons, event.MinerAddr)
		}
	}
	rt.StateTransaction(&st, func() {
		claims, err := adt.AsMap(adt.AsStore(rt), st.Claims)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load claims")

		// Remove power and leave miner frozen
		for _, minerAddr := range failedMinerCrons {
			claim, found, err := getClaim(claims, minerAddr)
			if err != nil {
				rt.Log(rtt.ERROR, "failed to get claim for miner %s after failing OnDeferredCronEvent: %s", minerAddr, err)
				continue
			}
			if !found {
				rt.Log(rtt.WARN, "miner OnDeferredCronEvent failed for miner %s with no power", minerAddr)
				continue
			}

			// zero out miner power
			err = st.addToClaim(claims, minerAddr, claim.RawBytePower.Neg(), claim.QualityAdjPower.Neg())
			if err != nil {
				rt.Log(rtt.WARN, "failed to remove (%d, %d) power for miner %s after to failed cron", claim.RawBytePower, claim.QualityAdjPower, minerAddr)
				continue
			}
		}

		st.Claims, err = claims.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to flush claims")
	})
}
