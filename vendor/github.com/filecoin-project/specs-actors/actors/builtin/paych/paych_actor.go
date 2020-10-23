package paych

import (
	"bytes"
	"math"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
)

// Maximum number of lanes in a channel.
const MaxLane = math.MaxInt64

const SettleDelay = builtin.EpochsInHour * 12

type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.UpdateChannelState,
		3:                         a.Settle,
		4:                         a.Collect,
	}
}

func (a Actor) Code() cid.Cid {
	return builtin.PaymentChannelActorCodeID
}

func (a Actor) State() cbor.Er {
	return new(State)
}

var _ runtime.VMActor = Actor{}

type ConstructorParams struct {
	From addr.Address // Payer
	To   addr.Address // Payee
}

// Constructor creates a payment channel actor. See State for meaning of params.
func (pca *Actor) Constructor(rt runtime.Runtime, params *ConstructorParams) *abi.EmptyValue {
	// Only InitActor can create a payment channel actor. It creates the actor on
	// behalf of the payer/payee.
	rt.ValidateImmediateCallerType(builtin.InitActorCodeID)

	// check that both parties are capable of signing vouchers
	to, err := pca.resolveAccount(rt, params.To)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to resolve to address: %s", params.To)
	from, err := pca.resolveAccount(rt, params.From)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to resolve from address: %s", params.From)

	emptyArrCid, err := adt.MakeEmptyArray(adt.AsStore(rt)).Root()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to create empty array")

	st := ConstructState(from, to, emptyArrCid)
	rt.StateCreate(st)

	return nil
}

// Resolves an address to a canonical ID address and requires it to address an account actor.
// The account actor constructor checks that the embedded address is associated with an appropriate key.
// An alternative (more expensive) would be to send a message to the actor to fetch its key.
func (pca *Actor) resolveAccount(rt runtime.Runtime, raw addr.Address) (addr.Address, error) {
	resolved, ok := rt.ResolveAddress(raw)
	if !ok {
		return addr.Undef, exitcode.ErrNotFound.Wrapf("failed to resolve address %v", raw)
	}

	codeCID, ok := rt.GetActorCodeCID(resolved)
	if !ok {
		return addr.Undef, exitcode.ErrForbidden.Wrapf("no code for address %v", resolved)
	}
	if codeCID != builtin.AccountActorCodeID {
		return addr.Undef, exitcode.ErrForbidden.Wrapf("actor %v must be an account (%v), was %v", raw,
			builtin.AccountActorCodeID, codeCID)
	}
	return resolved, nil
}

////////////////////////////////////////////////////////////////////////////////
// Payment Channel state operations
////////////////////////////////////////////////////////////////////////////////

type UpdateChannelStateParams struct {
	Sv     SignedVoucher
	Secret []byte
	Proof  []byte
}

// A voucher is sent by `From` to `To` off-chain in order to enable
// `To` to redeem payments on-chain in the future
type SignedVoucher struct {
	// ChannelAddr is the address of the payment channel this signed voucher is valid for
	ChannelAddr addr.Address
	// TimeLockMin sets a min epoch before which the voucher cannot be redeemed
	TimeLockMin abi.ChainEpoch
	// TimeLockMax sets a max epoch beyond which the voucher cannot be redeemed
	// TimeLockMax set to 0 means no timeout
	TimeLockMax abi.ChainEpoch
	// (optional) The SecretPreImage is used by `To` to validate
	SecretPreimage []byte
	// (optional) Extra can be specified by `From` to add a verification method to the voucher
	Extra *ModVerifyParams
	// Specifies which lane the Voucher merges into (will be created if does not exist)
	Lane uint64
	// Nonce is set by `From` to prevent redemption of stale vouchers on a lane
	Nonce uint64
	// Amount voucher can be redeemed for
	Amount big.Int
	// (optional) MinSettleHeight can extend channel MinSettleHeight if needed
	MinSettleHeight abi.ChainEpoch

	// (optional) Set of lanes to be merged into `Lane`
	Merges []Merge

	// Sender's signature over the voucher
	Signature *crypto.Signature
}

// Modular Verification method
type ModVerifyParams struct {
	Actor  addr.Address
	Method abi.MethodNum
	Data   []byte
}

type PaymentVerifyParams struct {
	Extra []byte
	Proof []byte
}

func (pca Actor) UpdateChannelState(rt runtime.Runtime, params *UpdateChannelStateParams) *abi.EmptyValue {
	var st State
	rt.StateReadonly(&st)

	// both parties must sign voucher: one who submits it, the other explicitly signs it
	rt.ValidateImmediateCallerIs(st.From, st.To)
	var signer addr.Address
	if rt.Caller() == st.From {
		signer = st.To
	} else {
		signer = st.From
	}
	sv := params.Sv

	if sv.Signature == nil {
		rt.Abortf(exitcode.ErrIllegalArgument, "voucher has no signature")
	}

	vb, err := sv.SigningBytes()
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "failed to serialize signedvoucher")

	err = rt.VerifySignature(*sv.Signature, signer, vb)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalArgument, "voucher signature invalid")

	pchAddr := rt.Receiver()
	if pchAddr != sv.ChannelAddr {
		rt.Abortf(exitcode.ErrIllegalArgument, "voucher payment channel address %s does not match receiver %s", sv.ChannelAddr, pchAddr)
	}

	if rt.CurrEpoch() < sv.TimeLockMin {
		rt.Abortf(exitcode.ErrIllegalArgument, "cannot use this voucher yet!")
	}

	if sv.TimeLockMax != 0 && rt.CurrEpoch() > sv.TimeLockMax {
		rt.Abortf(exitcode.ErrIllegalArgument, "this voucher has expired!")
	}

	if sv.Amount.Sign() < 0 {
		rt.Abortf(exitcode.ErrIllegalArgument, "voucher amount must be non-negative, was %v", sv.Amount)
	}

	if len(sv.SecretPreimage) > 0 {
		hashedSecret := rt.HashBlake2b(params.Secret)
		if !bytes.Equal(hashedSecret[:], sv.SecretPreimage) {
			rt.Abortf(exitcode.ErrIllegalArgument, "incorrect secret!")
		}
	}

	if sv.Extra != nil {

		code := rt.Send(
			sv.Extra.Actor,
			sv.Extra.Method,
			&PaymentVerifyParams{
				sv.Extra.Data,
				params.Proof,
			},
			abi.NewTokenAmount(0),
			&builtin.Discard{},
		)
		builtin.RequireSuccess(rt, code, "spend voucher verification failed")
	}

	rt.StateTransaction(&st, func() {
		laneFound := true

		lstates, err := adt.AsArray(adt.AsStore(rt), st.LaneStates)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load lanes")

		// Find the voucher lane, creating if necessary.
		laneId := sv.Lane
		laneState := findLane(rt, lstates, sv.Lane)

		if laneState == nil {
			laneState = &LaneState{
				Redeemed: big.Zero(),
				Nonce:    0,
			}
			laneFound = false
		}

		if laneFound {
			if laneState.Nonce >= sv.Nonce {
				rt.Abortf(exitcode.ErrIllegalArgument, "voucher has an outdated nonce, existing nonce: %d, voucher nonce: %d, cannot redeem",
					laneState.Nonce, sv.Nonce)
			}
		}

		// The next section actually calculates the payment amounts to update the payment channel state
		// 1. (optional) sum already redeemed value of all merging lanes
		redeemedFromOthers := big.Zero()
		for _, merge := range sv.Merges {
			if merge.Lane == sv.Lane {
				rt.Abortf(exitcode.ErrIllegalArgument, "voucher cannot merge lanes into its own lane")
			}

			otherls := findLane(rt, lstates, merge.Lane)
			if otherls == nil {
				rt.Abortf(exitcode.ErrIllegalArgument, "voucher specifies invalid merge lane %v", merge.Lane)
				return // makes linters happy
			}

			if otherls.Nonce >= merge.Nonce {
				rt.Abortf(exitcode.ErrIllegalArgument, "merged lane in voucher has outdated nonce, cannot redeem")
			}

			redeemedFromOthers = big.Add(redeemedFromOthers, otherls.Redeemed)
			otherls.Nonce = merge.Nonce
			err = lstates.Set(merge.Lane, otherls)
			builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to store lane %d", merge.Lane)
		}

		// 2. To prevent double counting, remove already redeemed amounts (from
		// voucher or other lanes) from the voucher amount
		laneState.Nonce = sv.Nonce
		balanceDelta := big.Sub(sv.Amount, big.Add(redeemedFromOthers, laneState.Redeemed))
		// 3. set new redeemed value for merged-into lane
		laneState.Redeemed = sv.Amount

		newSendBalance := big.Add(st.ToSend, balanceDelta)

		// 4. check operation validity
		if newSendBalance.LessThan(big.Zero()) {
			rt.Abortf(exitcode.ErrIllegalArgument, "voucher would leave channel balance negative")
		}
		if newSendBalance.GreaterThan(rt.CurrentBalance()) {
			rt.Abortf(exitcode.ErrIllegalArgument, "not enough funds in channel to cover voucher")
		}

		// 5. add new redemption ToSend
		st.ToSend = newSendBalance

		// update channel settlingAt and MinSettleHeight if delayed by voucher
		if sv.MinSettleHeight != 0 {
			if st.SettlingAt != 0 && st.SettlingAt < sv.MinSettleHeight {
				st.SettlingAt = sv.MinSettleHeight
			}
			if st.MinSettleHeight < sv.MinSettleHeight {
				st.MinSettleHeight = sv.MinSettleHeight
			}
		}

		err = lstates.Set(laneId, laneState)
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to store lane", laneId)

		st.LaneStates, err = lstates.Root()
		builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to save lanes")
	})
	return nil
}

func (pca Actor) Settle(rt runtime.Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	var st State
	rt.StateTransaction(&st, func() {
		rt.ValidateImmediateCallerIs(st.From, st.To)

		if st.SettlingAt != 0 {
			rt.Abortf(exitcode.ErrIllegalState, "channel already settling")
		}

		st.SettlingAt = rt.CurrEpoch() + SettleDelay
		if st.SettlingAt < st.MinSettleHeight {
			st.SettlingAt = st.MinSettleHeight
		}
	})
	return nil
}

func (pca Actor) Collect(rt runtime.Runtime, _ *abi.EmptyValue) *abi.EmptyValue {
	var st State
	rt.StateReadonly(&st)
	rt.ValidateImmediateCallerIs(st.From, st.To)

	if st.SettlingAt == 0 || rt.CurrEpoch() < st.SettlingAt {
		rt.Abortf(exitcode.ErrForbidden, "payment channel not settling or settled")
	}

	// send ToSend to "To"
	codeTo := rt.Send(
		st.To,
		builtin.MethodSend,
		nil,
		st.ToSend,
		&builtin.Discard{},
	)
	builtin.RequireSuccess(rt, codeTo, "Failed to send funds to `To`")

	// the remaining balance will be returned to "From" upon deletion.
	rt.DeleteActor(st.From)

	return nil
}

func (t *SignedVoucher) SigningBytes() ([]byte, error) {
	osv := *t
	osv.Signature = nil

	buf := new(bytes.Buffer)
	if err := osv.MarshalCBOR(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Returns the insertion index for a lane ID, with the matching lane state if found, or nil.
func findLane(rt runtime.Runtime, ls *adt.Array, id uint64) *LaneState {
	if id > MaxLane {
		rt.Abortf(exitcode.ErrIllegalArgument, "maximum lane ID is 2^63-1")
	}

	var out LaneState
	found, err := ls.Get(id, &out)
	builtin.RequireNoErr(rt, err, exitcode.ErrIllegalState, "failed to load lane %d", id)

	if !found {
		return nil
	}

	return &out
}
