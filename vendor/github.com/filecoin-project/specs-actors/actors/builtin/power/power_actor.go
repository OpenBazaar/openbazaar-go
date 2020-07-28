package power

import (
	"bytes"

	"github.com/filecoin-project/go-address"
	addr "github.com/filecoin-project/go-address"
	errors "github.com/pkg/errors"
	xerrors "golang.org/x/xerrors"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	big "github.com/filecoin-project/specs-actors/actors/abi/big"
	builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	initact "github.com/filecoin-project/specs-actors/actors/builtin/init"
	vmr "github.com/filecoin-project/specs-actors/actors/runtime"
	exitcode "github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	. "github.com/filecoin-project/specs-actors/actors/util"
	adt "github.com/filecoin-project/specs-actors/actors/util/adt"
)

type Runtime = vmr.Runtime

type SectorTermination int64

const (
	SectorTerminationExpired SectorTermination = iota // Implicit termination after all deals expire
	SectorTerminationManual                           // Unscheduled explicit termination by the miner
	SectorTerminationFaulty                           // Implicit termination due to unrecovered fault
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

var _ abi.Invokee = Actor{}

// Storage miner actor constructor params are defined here so the power actor can send them to the init actor
// to instantiate miners.
type MinerConstructorParams struct {
	OwnerAddr     addr.Address
	WorkerAddr    addr.Address
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

func (a Actor) Constructor(rt Runtime, _ *adt.EmptyValue) *adt.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.SystemActorAddr)

	emptyMap, err := adt.MakeEmptyMap(adt.AsStore(rt)).Root()
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to create storage power state: %v", err)
	}
	emptyMMapCid, err := adt.MakeEmptyMultimap(adt.AsStore(rt)).Root()
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to get empty multimap cid")
	}

	st := ConstructState(emptyMap, emptyMMapCid)
	rt.State().Create(st)
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
	if err != nil {
		rt.Abortf(exitcode.ErrPlaceholder, "failed to serialize miner constructor params %v: %v", ctorParams, err)
	}
	ret, code := rt.Send(
		builtin.InitActorAddr,
		builtin.MethodsInit.Exec,
		&initact.ExecParams{
			CodeCID:           builtin.StorageMinerActorCodeID,
			ConstructorParams: ctorParamBuf.Bytes(),
		},
		rt.Message().ValueReceived(), // Pass on any value to the new actor.
	)
	builtin.RequireSuccess(rt, code, "failed to init new actor")
	var addresses initact.ExecReturn
	err = ret.Into(&addresses)
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "unmarshaling exec return value: %v", err)
	}

	var st State
	rt.State().Transaction(&st, func() interface{} {
		store := adt.AsStore(rt)
		err = st.setClaim(store, addresses.IDAddress, &Claim{abi.NewStoragePower(0), abi.NewStoragePower(0)})
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to put power in claimed table while creating miner: %v", err)
		}
		st.MinerCount += 1
		return nil
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
func (a Actor) UpdateClaimedPower(rt Runtime, params *UpdateClaimedPowerParams) *adt.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)
	minerAddr := rt.Message().Caller()
	var st State
	rt.State().Transaction(&st, func() interface{} {
		err := st.AddToClaim(adt.AsStore(rt), minerAddr, params.RawByteDelta, params.QualityAdjustedDelta)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to update power raw %s, qa %s", params.RawByteDelta, params.QualityAdjustedDelta)
		return nil
	})
	return nil
}

type EnrollCronEventParams struct {
	EventEpoch abi.ChainEpoch
	Payload    []byte
}

func (a Actor) EnrollCronEvent(rt Runtime, params *EnrollCronEventParams) *adt.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)
	minerAddr := rt.Message().Caller()
	minerEvent := CronEvent{
		MinerAddr:       minerAddr,
		CallbackPayload: params.Payload,
	}

	var st State
	rt.State().Transaction(&st, func() interface{} {
		err := st.appendCronEvent(adt.AsStore(rt), params.EventEpoch, &minerEvent)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to enroll cron event: %v", err)
		}
		return nil
	})
	return nil
}

// Called by Cron.
func (a Actor) OnEpochTickEnd(rt Runtime, _ *adt.EmptyValue) *adt.EmptyValue {
	rt.ValidateImmediateCallerIs(builtin.CronActorAddr)

	if err := a.processDeferredCronEvents(rt); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "Failed to process deferred cron events: %v", err)
	}

	if err := a.processBatchProofVerifies(rt); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to process batch proof verification: %s", err)
	}

	var st State
	rt.State().Readonly(&st)

	// update network KPI in RewardActor
	_, code := rt.Send(
		builtin.RewardActorAddr,
		builtin.MethodsReward.UpdateNetworkKPI,
		&st.TotalRawBytePower,
		abi.NewTokenAmount(0),
	)
	builtin.RequireSuccess(rt, code, "failed to update network KPI with Reward Actor")

	return nil
}

func (a Actor) UpdatePledgeTotal(rt Runtime, pledgeDelta *abi.TokenAmount) *adt.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)
	var st State
	rt.State().Transaction(&st, func() interface{} {
		st.addPledgeTotal(*pledgeDelta)
		return nil
	})
	return nil
}

func (a Actor) OnConsensusFault(rt Runtime, pledgeAmount *abi.TokenAmount) *adt.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)
	minerAddr := rt.Message().Caller()

	var st State
	rt.State().Transaction(&st, func() interface{} {
		claim, powerOk, err := st.GetClaim(adt.AsStore(rt), minerAddr)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to read claimed power for fault: %v", err)
		}
		if !powerOk {
			rt.Abortf(exitcode.ErrIllegalArgument, "miner %v not registered (already slashed?)", minerAddr)
		}
		Assert(claim.RawBytePower.GreaterThanEqual(big.Zero()))
		Assert(claim.QualityAdjPower.GreaterThanEqual(big.Zero()))

		st.TotalQualityAdjPower = big.Sub(st.TotalQualityAdjPower, claim.QualityAdjPower)
		st.TotalRawBytePower = big.Sub(st.TotalRawBytePower, claim.RawBytePower)

		st.addPledgeTotal(pledgeAmount.Neg())
		return nil
	})

	err := a.deleteMinerActor(rt, minerAddr)
	AssertNoError(err)

	return nil
}

// GasOnSubmitVerifySeal is amount of gas charged for SubmitPoRepForBulkVerify
// This number is empirically determined
const GasOnSubmitVerifySeal = 132166313

func (a Actor) SubmitPoRepForBulkVerify(rt Runtime, sealInfo *abi.SealVerifyInfo) *adt.EmptyValue {
	rt.ValidateImmediateCallerType(builtin.StorageMinerActorCodeID)

	minerAddr := rt.Message().Caller()

	rt.ChargeGas("OnSubmitVerifySeal", GasOnSubmitVerifySeal, 0)
	var st State
	rt.State().Transaction(&st, func() interface{} {
		store := adt.AsStore(rt)
		var mmap *adt.Multimap
		if st.ProofValidationBatch == nil {
			mmap = adt.MakeEmptyMultimap(store)
		} else {
			var err error
			mmap, err = adt.AsMultimap(adt.AsStore(rt), *st.ProofValidationBatch)
			if err != nil {
				rt.Abortf(exitcode.ErrIllegalState, "failed to load proof batching set: %s", err)
			}
		}

		if err := mmap.Add(adt.AddrKey(minerAddr), sealInfo); err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to insert proof into set: %s", err)
		}

		mmrc, err := mmap.Root()
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to flush proofs batch map: %s", err)
		}
		st.ProofValidationBatch = &mmrc
		return nil
	})

	return nil
}

type CurrentTotalPowerReturn struct {
	RawBytePower     abi.StoragePower
	QualityAdjPower  abi.StoragePower
	PledgeCollateral abi.TokenAmount
}

// Returns the total power and pledge recorded by the power actor.
// TODO hold these values constant during an epoch for stable calculations, https://github.com/filecoin-project/specs-actors/issues/495
func (a Actor) CurrentTotalPower(rt Runtime, _ *adt.EmptyValue) *CurrentTotalPowerReturn {
	rt.ValidateImmediateCallerAcceptAny()
	var st State
	rt.State().Readonly(&st)
	return &CurrentTotalPowerReturn{
		RawBytePower:     st.TotalRawBytePower,
		QualityAdjPower:  st.TotalQualityAdjPower,
		PledgeCollateral: st.TotalPledgeCollateral,
	}
}

////////////////////////////////////////////////////////////////////////////////
// Method utility functions
////////////////////////////////////////////////////////////////////////////////

func (a Actor) processBatchProofVerifies(rt Runtime) error {
	var st State

	var miners []address.Address
	verifies := make(map[address.Address][]abi.SealVerifyInfo)

	rt.State().Transaction(&st, func() interface{} {
		store := adt.AsStore(rt)
		if st.ProofValidationBatch == nil {
			return nil
		}
		mmap, err := adt.AsMultimap(store, *st.ProofValidationBatch)
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to load proofs validation batch: %s", err)
		}

		err = mmap.ForAll(func(k string, arr *adt.Array) error {
			a, err := address.NewFromBytes([]byte(k))
			if err != nil {
				return xerrors.Errorf("failed to parse address key: %w", err)
			}

			miners = append(miners, a)

			var infos []abi.SealVerifyInfo
			var svi abi.SealVerifyInfo
			err = arr.ForEach(&svi, func(i int64) error {
				infos = append(infos, svi)
				return nil
			})
			if err != nil {
				return xerrors.Errorf("failed to iterate over proof verify array for miner %s: %w", a, err)
			}
			verifies[a] = infos
			return nil
		})
		if err != nil {
			rt.Abortf(exitcode.ErrIllegalState, "failed to iterate proof batch: %s", err)
		}

		st.ProofValidationBatch = nil

		return nil
	})

	res, err := rt.Syscalls().BatchVerifySeals(verifies)
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to batch verify: %s", err)
	}

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
		_, _ = rt.Send(
			m,
			builtin.MethodsMiner.ConfirmSectorProofsValid,
			&builtin.ConfirmSectorProofsParams{Sectors: successful},
			abi.NewTokenAmount(0),
		)
	}

	return nil
}

func (a Actor) processDeferredCronEvents(rt Runtime) error {
	rtEpoch := rt.CurrEpoch()

	var cronEvents []CronEvent
	var st State
	rt.State().Transaction(&st, func() interface{} {
		store := adt.AsStore(rt)

		for epoch := st.LastEpochTick + 1; epoch <= rtEpoch; epoch++ {
			epochEvents, err := st.loadCronEvents(store, epoch)
			if err != nil {
				return errors.Wrapf(err, "failed to load cron events at %v", epoch)
			}

			cronEvents = append(cronEvents, epochEvents...)

			if len(epochEvents) > 0 {
				err = st.clearCronEvents(store, epoch)
				if err != nil {
					return errors.Wrapf(err, "failed to clear cron events at %v", epoch)
				}
			}
		}

		st.LastEpochTick = rtEpoch
		return nil
	})
	failedMinerCrons := make([]addr.Address, 0)
	for _, event := range cronEvents {
		_, code := rt.Send(
			event.MinerAddr,
			builtin.MethodsMiner.OnDeferredCronEvent,
			vmr.CBORBytes(event.CallbackPayload),
			abi.NewTokenAmount(0),
		)
		// If a callback fails, this actor continues to invoke other callbacks
		// and persists state removing the failed event from the event queue. It won't be tried again.
		// Failures are unexpected here but will result in removal of miner power
		// A log message would really help here.
		if code != exitcode.Ok {
			rt.Log(vmr.WARN, "OnDeferredCronEvent failed for miner %s: exitcode %d", event.MinerAddr, code)
			failedMinerCrons = append(failedMinerCrons, event.MinerAddr)
		}
	}
	rt.State().Transaction(&st, func() interface{} {
		store := adt.AsStore(rt)
		// Remove power and leave miner frozen
		for _, minerAddr := range failedMinerCrons {
			claim, found, err := st.GetClaim(store, minerAddr)
			if err != nil {
				rt.Log(vmr.ERROR, "failed to get claim for miner %s after failing OnDeferredCronEvent: %s", minerAddr, err)
				continue
			}
			if !found {
				rt.Log(vmr.WARN, "miner OnDeferredCronEvent failed for miner %s with no power", minerAddr)
				continue
			}

			// zero out miner power
			err = st.AddToClaim(store, minerAddr, claim.RawBytePower.Neg(), claim.QualityAdjPower.Neg())
			if err != nil {
				rt.Log(vmr.WARN, "failed to remove (%d, %d) power for miner %s after to failed cron", claim.RawBytePower, claim.QualityAdjPower, minerAddr)
				continue
			}
		}
		return nil
	})
	return nil
}

func (a Actor) deleteMinerActor(rt Runtime, miner addr.Address) error {
	var st State
	err := rt.State().Transaction(&st, func() interface{} {
		if err := st.deleteClaim(adt.AsStore(rt), miner); err != nil {
			return errors.Wrapf(err, "failed to delete %v from claimed power table", miner)
		}

		st.MinerCount -= 1
		return nil
	}).(error)
	return err
}
