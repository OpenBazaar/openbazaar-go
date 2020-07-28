package multisig_test

import (
	"bytes"
	"context"
	"testing"

	addr "github.com/filecoin-project/go-address"
	"github.com/minio/blake2b-simd"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	big "github.com/filecoin-project/specs-actors/actors/abi/big"
	builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	miner "github.com/filecoin-project/specs-actors/actors/builtin/miner"
	multisig "github.com/filecoin-project/specs-actors/actors/builtin/multisig"
	runtime "github.com/filecoin-project/specs-actors/actors/runtime"
	exitcode "github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	adt "github.com/filecoin-project/specs-actors/actors/util/adt"
	mock "github.com/filecoin-project/specs-actors/support/mock"
	tutil "github.com/filecoin-project/specs-actors/support/testing"
)

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, multisig.Actor{})
}

func TestConstruction(t *testing.T) {
	actor := multisig.Actor{}

	receiver := tutil.NewIDAddr(t, 100)
	anne := tutil.NewIDAddr(t, 101)
	bob := tutil.NewIDAddr(t, 102)
	charlie := tutil.NewIDAddr(t, 103)

	builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("simple construction", func(t *testing.T) {
		rt := builder.Build(t)
		params := multisig.ConstructorParams{
			Signers:               []addr.Address{anne, bob, charlie},
			NumApprovalsThreshold: 2,
			UnlockDuration:        0,
		}

		rt.ExpectValidateCallerAddr(builtin.InitActorAddr)
		ret := rt.Call(actor.Constructor, &params)
		assert.Nil(t, ret)
		rt.Verify()

		var st multisig.State
		rt.GetState(&st)
		assert.Equal(t, params.Signers, st.Signers)
		assert.Equal(t, params.NumApprovalsThreshold, st.NumApprovalsThreshold)
		assert.Equal(t, abi.NewTokenAmount(0), st.InitialBalance)
		assert.Equal(t, abi.ChainEpoch(0), st.UnlockDuration)
		assert.Equal(t, abi.ChainEpoch(0), st.StartEpoch)
		txns, err := adt.AsMap(adt.AsStore(rt), st.PendingTxns)
		assert.NoError(t, err)
		keys, err := txns.CollectKeys()
		require.NoError(t, err)
		assert.Empty(t, keys)
	})

	t.Run("construction with vesting", func(t *testing.T) {
		rt := builder.WithEpoch(1234).Build(t)
		params := multisig.ConstructorParams{
			Signers:               []addr.Address{anne, bob, charlie},
			NumApprovalsThreshold: 3,
			UnlockDuration:        100,
		}
		rt.ExpectValidateCallerAddr(builtin.InitActorAddr)
		ret := rt.Call(actor.Constructor, &params)
		assert.Nil(t, ret)
		rt.Verify()

		var st multisig.State
		rt.GetState(&st)
		assert.Equal(t, params.Signers, st.Signers)
		assert.Equal(t, params.NumApprovalsThreshold, st.NumApprovalsThreshold)
		assert.Equal(t, abi.NewTokenAmount(0), st.InitialBalance)
		assert.Equal(t, abi.ChainEpoch(100), st.UnlockDuration)
		assert.Equal(t, abi.ChainEpoch(1234), st.StartEpoch)
		// assert no transactions
	})

	t.Run("fail to construct multisig actor with 0 signers", func(t *testing.T) {
		rt := builder.Build(t)
		params := multisig.ConstructorParams{
			Signers:               []addr.Address{},
			NumApprovalsThreshold: 1,
			UnlockDuration:        1,
		}
		rt.ExpectValidateCallerAddr(builtin.InitActorAddr)
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			rt.Call(actor.Constructor, &params)
		})
		rt.Verify()

	})

	t.Run("fail to construct multisig with more approvals than signers", func(t *testing.T) {
		rt := builder.Build(t)
		params := multisig.ConstructorParams{
			Signers:               []addr.Address{anne, bob, charlie},
			NumApprovalsThreshold: 4,
			UnlockDuration:        1,
		}
		rt.ExpectValidateCallerAddr(builtin.InitActorAddr)
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			rt.Call(actor.Constructor, &params)
		})
		rt.Verify()
	})

	t.Run("fail to construct multisig with duplicate signers(all ID addresses)", func(t *testing.T) {
		rt := builder.Build(t)
		params := multisig.ConstructorParams{
			Signers:               []addr.Address{anne, bob, bob},
			NumApprovalsThreshold: 2,
			UnlockDuration:        0,
		}

		rt.ExpectValidateCallerAddr(builtin.InitActorAddr)
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			rt.Call(actor.Constructor, &params)
		})
		rt.Verify()
	})

	t.Run("fail to construct multisig with duplicate signers(ID & non-ID addresses)", func(t *testing.T) {
		bobNonId := tutil.NewBLSAddr(t,1)
		rt := builder.Build(t)
		params := multisig.ConstructorParams{
			Signers:               []addr.Address{anne, bobNonId, bob},
			NumApprovalsThreshold: 2,
			UnlockDuration:        0,
		}

		rt.AddIDAddress(bobNonId, bob)
		rt.ExpectValidateCallerAddr(builtin.InitActorAddr)
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			rt.Call(actor.Constructor, &params)
		})
		rt.Verify()
	})
}

func TestVesting(t *testing.T) {
	actor := msActorHarness{multisig.Actor{}, t}

	receiver := tutil.NewIDAddr(t, 100)
	anne := tutil.NewIDAddr(t, 101)
	bob := tutil.NewIDAddr(t, 102)
	charlie := tutil.NewIDAddr(t, 103)
	darlene := tutil.NewIDAddr(t, 103)

	const unlockDuration = 10
	var multisigInitialBalance = abi.NewTokenAmount(100)
	var fakeParams = runtime.CBORBytes([]byte{1, 2, 3, 4})

	builder := mock.NewBuilder(context.Background(), receiver).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID).
		WithEpoch(0).
		// balance 0: current balance of the actor. receive: 100 the amount the multisig actor will be initalized with -- InitialBalance
		WithBalance(multisigInitialBalance, multisigInitialBalance).
		WithHasher(blake2b.Sum256)

	t.Run("happy path full vesting", func(t *testing.T) {
		rt := builder.Build(t)

		actor.constructAndVerify(rt, 2, unlockDuration, []addr.Address{anne, bob, charlie}...)

		// anne proposes that darlene receives `multisgiInitialBalance` FIL.
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.SetReceived(big.Zero())
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, darlene, multisigInitialBalance, builtin.MethodSend, fakeParams, nil)
		rt.Verify()

		// Advance the epoch s.t. all funds are unlocked.
		rt.SetEpoch(0 + unlockDuration)
		// bob approves annes transaction
		rt.SetCaller(bob, builtin.AccountActorCodeID)
		// expect darlene to receive the transaction proposed by anne.
		rt.ExpectSend(darlene, builtin.MethodSend, fakeParams, multisigInitialBalance, nil, exitcode.Ok)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		proposalHashData := makeProposalHash(t, &multisig.Transaction{
			To:       darlene,
			Value:    multisigInitialBalance,
			Method:   builtin.MethodSend,
			Params:   fakeParams,
			Approved: []addr.Address{anne},
		})
		actor.approveOK(rt, 0, proposalHashData, nil)
	})

	t.Run("partial vesting propose to send half the actor balance when the epoch is hald the unlock duration", func(t *testing.T) {
		rt := builder.Build(t)

		actor.constructAndVerify(rt, 2, 10, []addr.Address{anne, bob, charlie}...)

		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.SetReceived(big.Zero())
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, darlene, big.Div(multisigInitialBalance, big.NewInt(2)), builtin.MethodSend, fakeParams, nil)
		rt.Verify()

		// set the current balance of the multisig actor to its InitialBalance amount
		rt.SetEpoch(0 + unlockDuration/2)
		rt.SetCaller(bob, builtin.AccountActorCodeID)
		rt.ExpectSend(darlene, builtin.MethodSend, fakeParams, big.Div(multisigInitialBalance, big.NewInt(2)), nil, exitcode.Ok)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)

		proposalHashData := makeProposalHash(t, &multisig.Transaction{
			To:       darlene,
			Value:    big.Div(multisigInitialBalance, big.NewInt(2)),
			Method:   builtin.MethodSend,
			Params:   fakeParams,
			Approved: []addr.Address{anne},
		})

		actor.approveOK(rt, 0, proposalHashData, nil)
	})

	t.Run("propose and autoapprove transaction above locked amount fails", func(t *testing.T) {
		rt := builder.Build(t)

		actor.constructAndVerify(rt, 1, unlockDuration, []addr.Address{anne, bob, charlie}...)

		rt.SetReceived(big.Zero())
		// this propose will fail since it would send more than the required locked balance and num approvals == 1
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectAbort(exitcode.ErrInsufficientFunds, func() {
			_ = actor.propose(rt, darlene, abi.NewTokenAmount(100), builtin.MethodSend, fakeParams, nil)
		})
		rt.Verify()

		// this will pass since sending below the locked amount is permitted
		rt.SetEpoch(1)
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectSend(darlene, builtin.MethodSend, fakeParams, abi.NewTokenAmount(10), nil, 0)
		actor.proposeOK(rt, darlene, abi.NewTokenAmount(10), builtin.MethodSend, fakeParams, nil)
		rt.Verify()
	})

	t.Run("fail to vest more than locked amount", func(t *testing.T) {
		rt := builder.Build(t)

		actor.constructAndVerify(rt, 2, unlockDuration, []addr.Address{anne, bob, charlie}...)

		rt.SetReceived(big.Zero())
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, darlene, big.Div(multisigInitialBalance, big.NewInt(2)), builtin.MethodSend, fakeParams, nil)
		rt.Verify()

		// this propose will fail since it would send more than the required locked balance.
		rt.SetEpoch(1)
		rt.SetCaller(bob, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectAbort(exitcode.ErrInsufficientFunds, func() {
			proposalHashData := makeProposalHash(t, &multisig.Transaction{
				To:       darlene,
				Value:    big.Div(multisigInitialBalance, big.NewInt(2)),
				Method:   builtin.MethodSend,
				Params:   fakeParams,
				Approved: []addr.Address{anne},
			})
			_ = actor.approve(rt, 0, proposalHashData, nil)
		})
		rt.Verify()
	})

}

func TestPropose(t *testing.T) {
	actor := msActorHarness{multisig.Actor{}, t}

	receiver := tutil.NewIDAddr(t, 100)
	anne := tutil.NewIDAddr(t, 101)
	bob := tutil.NewIDAddr(t, 102)
	chuck := tutil.NewIDAddr(t, 103)

	const noUnlockDuration = int64(0)
	var sendValue = abi.NewTokenAmount(10)
	var fakeParams = runtime.CBORBytes([]byte{1, 2, 3, 4})
	var signers = []addr.Address{anne, bob}

	builder := mock.NewBuilder(context.Background(), receiver).WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)

	t.Run("simple propose", func(t *testing.T) {
		const numApprovals = uint64(2)
		rt := builder.Build(t)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, chuck, sendValue, builtin.MethodSend, fakeParams, nil)

		// the transaction remains awaiting second approval
		actor.assertTransactions(rt, multisig.Transaction{
			To:       chuck,
			Value:    sendValue,
			Method:   builtin.MethodSend,
			Params:   fakeParams,
			Approved: []addr.Address{anne},
		})
	})

	t.Run("propose with threshold met", func(t *testing.T) {
		const numApprovals = uint64(1)

		rt := builder.WithBalance(abi.NewTokenAmount(20), abi.NewTokenAmount(0)).Build(t)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		rt.ExpectSend(chuck, builtin.MethodSend, fakeParams, sendValue, nil, 0)

		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, chuck, sendValue, builtin.MethodSend, fakeParams, nil)

		// the transaction has been sent and cleaned up
		actor.assertTransactions(rt)
		rt.Verify()
	})

	t.Run("propose with threshold and non-empty return value", func(t *testing.T) {
		const numApprovals = uint64(1)

		rt := builder.WithBalance(abi.NewTokenAmount(20), abi.NewTokenAmount(0)).Build(t)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		proposeRet := miner.GetControlAddressesReturn{
			Owner:  tutil.NewIDAddr(t, 1),
			Worker: tutil.NewIDAddr(t, 2),
		}
		rt.ExpectSend(chuck, builtin.MethodsMiner.ControlAddresses, fakeParams, sendValue, &proposeRet, 0)

		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)

		var out miner.GetControlAddressesReturn
		actor.proposeOK(rt, chuck, sendValue, builtin.MethodsMiner.ControlAddresses, fakeParams, &out)
		// assert ProposeReturn.Ret can be marshaled into the expected structure.
		assert.Equal(t, proposeRet, out)

		// the transaction has been sent and cleaned up
		actor.assertTransactions(rt)
		rt.Verify()

	})

	t.Run("fail propose with threshold met and insufficient balance", func(t *testing.T) {
		const numApprovals = uint64(1)
		rt := builder.WithBalance(abi.NewTokenAmount(0), abi.NewTokenAmount(0)).Build(t)
		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectAbort(exitcode.ErrInsufficientFunds, func() {
			_ = actor.propose(rt, chuck, sendValue, builtin.MethodSend, fakeParams, nil)
		})
		rt.Verify()

		// proposal failed since it should have but failed to immediately execute.
		actor.assertTransactions(rt)
	})

	t.Run("fail propose from non-signer", func(t *testing.T) {
		// non-signer address
		richard := tutil.NewIDAddr(t, 105)
		const numApprovals = uint64(2)

		rt := builder.Build(t)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		rt.SetCaller(richard, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectAbort(exitcode.ErrForbidden, func() {
			_ = actor.propose(rt, chuck, sendValue, builtin.MethodSend, fakeParams, nil)
		})
		rt.Verify()

		// the transaction is not persisted
		actor.assertTransactions(rt)
	})
}

func TestApprove(t *testing.T) {
	actor := msActorHarness{multisig.Actor{}, t}

	receiver := tutil.NewIDAddr(t, 100)
	anne := tutil.NewIDAddr(t, 101)
	bob := tutil.NewIDAddr(t, 102)
	chuck := tutil.NewIDAddr(t, 103)

	const noUnlockDuration = int64(0)
	const numApprovals = uint64(2)
	const txnID = int64(0)
	const fakeMethod = abi.MethodNum(42)
	var sendValue = abi.NewTokenAmount(10)
	var fakeParams = runtime.CBORBytes([]byte{1, 2, 3, 4})
	var signers = []addr.Address{anne, bob}

	builder := mock.NewBuilder(context.Background(), receiver).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID).
		WithHasher(blake2b.Sum256)

	t.Run("simple propose and approval", func(t *testing.T) {
		rt := builder.Build(t)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, chuck, sendValue, fakeMethod, fakeParams, nil)
		rt.Verify()

		actor.assertTransactions(rt, multisig.Transaction{
			To:       chuck,
			Value:    sendValue,
			Method:   fakeMethod,
			Params:   fakeParams,
			Approved: []addr.Address{anne},
		})

		rt.SetBalance(sendValue)
		rt.SetCaller(bob, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectSend(chuck, fakeMethod, fakeParams, sendValue, nil, 0)

		proposalHashData := makeProposalHash(t, &multisig.Transaction{
			To:       chuck,
			Value:    sendValue,
			Method:   fakeMethod,
			Params:   fakeParams,
			Approved: []addr.Address{anne},
		})

		actor.approveOK(rt, txnID, proposalHashData, nil)

		// Transaction should be removed from actor state after send
		actor.assertTransactions(rt)
	})

	t.Run("approve with non-empty return value", func(t *testing.T) {
		const numApprovals = uint64(2)

		rt := builder.WithBalance(abi.NewTokenAmount(20), abi.NewTokenAmount(0)).Build(t)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, chuck, sendValue, builtin.MethodsMiner.ControlAddresses, fakeParams, nil)

		approveRet := miner.GetControlAddressesReturn{
			Owner:  tutil.NewIDAddr(t, 1),
			Worker: tutil.NewIDAddr(t, 2),
		}

		proposalHashData := makeProposalHash(t, &multisig.Transaction{
			To:       chuck,
			Value:    sendValue,
			Method:   builtin.MethodsMiner.ControlAddresses,
			Params:   fakeParams,
			Approved: []addr.Address{anne},
		})

		rt.SetCaller(bob, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectSend(chuck, builtin.MethodsMiner.ControlAddresses, fakeParams, sendValue, &approveRet, 0)

		var out miner.GetControlAddressesReturn
		actor.approveOK(rt, txnID, proposalHashData, &out)
		// assert approveRet.Ret can be marshaled into the expected structure.
		assert.Equal(t, approveRet, out)

		// the transaction has been sent and cleaned up
		actor.assertTransactions(rt)
	})

	t.Run("fail approval with bad proposal hash", func(t *testing.T) {
		rt := builder.Build(t)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, chuck, sendValue, fakeMethod, fakeParams, nil)
		rt.Verify()

		actor.assertTransactions(rt, multisig.Transaction{
			To:       chuck,
			Value:    sendValue,
			Method:   fakeMethod,
			Params:   fakeParams,
			Approved: []addr.Address{anne},
		})

		rt.SetBalance(sendValue)
		rt.SetCaller(bob, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectSend(chuck, fakeMethod, fakeParams, sendValue, nil, 0)

		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			proposalHashData := makeProposalHash(t, &multisig.Transaction{
				To:       chuck,
				Value:    sendValue,
				Method:   fakeMethod,
				Params:   fakeParams,
				Approved: []addr.Address{bob},
			})
			_ = actor.approve(rt, txnID, proposalHashData, nil)
		})
	})

	t.Run("fail approve transaction more than once", func(t *testing.T) {
		const numApprovals = uint64(2)
		rt := builder.Build(t)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, chuck, sendValue, builtin.MethodSend, fakeParams, nil)
		rt.Verify()

		// anne is going to approve it twice and fail, poor anne.
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectAbort(exitcode.ErrForbidden, func() {
			proposalHashData := makeProposalHash(t, &multisig.Transaction{
				To:       chuck,
				Value:    sendValue,
				Method:   builtin.MethodSend,
				Params:   fakeParams,
				Approved: []addr.Address{anne},
			})
			_ = actor.approve(rt, txnID, proposalHashData, nil)
		})
		rt.Verify()

		// Transaction still exists
		actor.assertTransactions(rt, multisig.Transaction{
			To:       chuck,
			Value:    sendValue,
			Method:   builtin.MethodSend,
			Params:   fakeParams,
			Approved: []addr.Address{anne},
		})
	})

	t.Run("fail approve transaction that does not exist", func(t *testing.T) {
		const dneTxnID = int64(1)
		rt := builder.Build(t)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, chuck, sendValue, builtin.MethodSend, fakeParams, nil)
		rt.Verify()

		// bob is going to approve a transaction that doesn't exist.
		rt.SetCaller(bob, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectAbort(exitcode.ErrNotFound, func() {
			proposalHashData := makeProposalHash(t, &multisig.Transaction{
				To:       chuck,
				Value:    sendValue,
				Method:   builtin.MethodSend,
				Params:   fakeParams,
				Approved: []addr.Address{bob},
			})
			_ = actor.approve(rt, dneTxnID, proposalHashData, nil)
		})
		rt.Verify()

		// Transaction was not removed from store.
		actor.assertTransactions(rt, multisig.Transaction{
			To:       chuck,
			Value:    sendValue,
			Method:   builtin.MethodSend,
			Params:   fakeParams,
			Approved: []addr.Address{anne},
		})
	})

	t.Run("fail to approve transaction by non-signer", func(t *testing.T) {
		// non-signer address
		richard := tutil.NewIDAddr(t, 105)
		rt := builder.Build(t)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, chuck, sendValue, builtin.MethodSend, fakeParams, nil)

		// richard is going to approve a transaction they are not a signer for.
		rt.SetCaller(richard, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectAbort(exitcode.ErrForbidden, func() {
			proposalHashData := makeProposalHash(t, &multisig.Transaction{
				To:       chuck,
				Value:    sendValue,
				Method:   builtin.MethodSend,
				Params:   fakeParams,
				Approved: []addr.Address{richard},
			})
			_ = actor.approve(rt, txnID, proposalHashData, nil)
		})
		rt.Verify()

		// Transaction was not removed from store.
		actor.assertTransactions(rt, multisig.Transaction{
			To:       chuck,
			Value:    sendValue,
			Method:   builtin.MethodSend,
			Params:   fakeParams,
			Approved: []addr.Address{anne},
		})
	})

	t.Run("proposed transaction is approved by proposer if number of approvers has already crossed threshold", func(t *testing.T) {
		rt := builder.Build(t)
		const newThreshold = 1
		signers := []addr.Address{anne, bob}
		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		// anne proposes a transaction
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		proposalHash := actor.proposeOK(rt, chuck, sendValue, fakeMethod, fakeParams, nil)
		rt.Verify()

		// reduce the threshold so the transaction is already approved
		rt.SetCaller(receiver, builtin.MultisigActorCodeID)
		rt.ExpectValidateCallerAddr(receiver)
		actor.changeNumApprovalsThreshold(rt, newThreshold)
		rt.Verify()

		// even if anne calls for an approval again(duplicate approval), transaction is executed because the threshold has been met.
		rt.ExpectSend(chuck, fakeMethod, fakeParams, sendValue, nil, 0)
		rt.SetBalance(sendValue)
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.approveOK(rt, txnID, proposalHash, nil)

		// Transaction should be removed from actor state after send
		actor.assertTransactions(rt)
	})

	t.Run("approve transaction if number of approvers has already crossed threshold even if we attempt a duplicate approval", func(t *testing.T) {
		rt := builder.Build(t)
		const numApprovals = 3
		const newThreshold = 2
		signers := []addr.Address{anne, bob, chuck}
		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		// anne proposes a transaction
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		proposalHash := actor.proposeOK(rt, chuck, sendValue, fakeMethod, fakeParams, nil)
		rt.Verify()

		// bob approves the transaction (number of approvals is now two but threshold is three)
		rt.SetCaller(bob, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)

		actor.approveOK(rt, txnID, proposalHash, nil)

		// reduce the threshold so the transaction is already approved
		rt.SetCaller(receiver, builtin.MultisigActorCodeID)
		rt.ExpectValidateCallerAddr(receiver)
		actor.changeNumApprovalsThreshold(rt, newThreshold)
		rt.Verify()

		// even if bob calls for an approval again(duplicate approval), transaction is executed because the threshold has been met.
		rt.ExpectSend(chuck, fakeMethod, fakeParams, sendValue, nil, 0)
		rt.SetBalance(sendValue)
		rt.SetCaller(bob, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.approveOK(rt, txnID, proposalHash, nil)

		// Transaction should be removed from actor state after send
		actor.assertTransactions(rt)
	})

	t.Run("approve transaction if number of approvers has already crossed threshold and ensure non-signatory cannot approve a transaction", func(t *testing.T) {
		rt := builder.Build(t)
		const newThreshold = 1
		signers := []addr.Address{anne, bob}
		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		// anne proposes a transaction
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		proposalHash := actor.proposeOK(rt, chuck, sendValue, fakeMethod, fakeParams, nil)
		rt.Verify()

		// reduce the threshold so the transaction is already approved
		rt.SetCaller(receiver, builtin.MultisigActorCodeID)
		rt.ExpectValidateCallerAddr(receiver)
		actor.changeNumApprovalsThreshold(rt, newThreshold)
		rt.Verify()

		// alice cannot approve the transaction as alice is not a signatory
		alice := tutil.NewIDAddr(t, 104)
		rt.SetCaller(alice, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectAbort(exitcode.ErrForbidden, func() {
			_ = actor.approve(rt, txnID, proposalHash, nil)
		})
		rt.Verify()

		// bob attempts to approve the transaction but it gets approved without
		// processing his approval as it the threshold has been met.
		rt.ExpectSend(chuck, fakeMethod, fakeParams, sendValue, nil, 0)
		rt.SetBalance(sendValue)
		rt.SetCaller(bob, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)

		actor.approveOK(rt, txnID, proposalHash, nil)

		// Transaction should be removed from actor state after send
		actor.assertTransactions(rt)
	})
}

func TestCancel(t *testing.T) {
	actor := msActorHarness{multisig.Actor{}, t}

	richard := tutil.NewIDAddr(t, 104)
	receiver := tutil.NewIDAddr(t, 100)
	anne := tutil.NewIDAddr(t, 101)
	bob := tutil.NewIDAddr(t, 102)
	chuck := tutil.NewIDAddr(t, 103)

	const noUnlockDuration = int64(0)
	const numApprovals = uint64(2)
	const txnID = int64(0)
	const fakeMethod = abi.MethodNum(42)
	var fakeParams = []byte{1, 2, 3, 4, 5}
	var sendValue = abi.NewTokenAmount(10)
	var signers = []addr.Address{anne, bob}

	builder := mock.NewBuilder(context.Background(), receiver).
		WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID).
		WithHasher(blake2b.Sum256)

	t.Run("simple propose and cancel", func(t *testing.T) {
		rt := builder.Build(t)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		// anne proposes a transaction
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, chuck, sendValue, fakeMethod, fakeParams, nil)
		rt.Verify()

		// anne cancels their transaction
		rt.SetBalance(sendValue)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)

		proposalHashData := makeProposalHash(t, &multisig.Transaction{
			To:       chuck,
			Value:    sendValue,
			Method:   fakeMethod,
			Params:   fakeParams,
			Approved: []addr.Address{anne},
		})
		actor.cancel(rt, txnID, proposalHashData)
		rt.Verify()

		// Transaction should be removed from actor state after cancel
		actor.assertTransactions(rt)
	})

	t.Run("fail cancel with bad proposal hash", func(t *testing.T) {
		rt := builder.Build(t)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		// anne proposes a transaction
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, chuck, sendValue, fakeMethod, fakeParams, nil)
		rt.Verify()

		// anne cancels their transaction
		rt.SetBalance(sendValue)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)

		rt.ExpectAbort(exitcode.ErrIllegalState, func() {
			proposalHashData := makeProposalHash(t, &multisig.Transaction{
				To:       bob,
				Value:    sendValue,
				Method:   fakeMethod,
				Params:   fakeParams,
				Approved: []addr.Address{chuck},
			})
			actor.cancel(rt, txnID, proposalHashData)
		})
	})

	t.Run("signer fails to cancel transaction from another signer", func(t *testing.T) {
		rt := builder.Build(t)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		// anne proposes a transaction
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, chuck, sendValue, fakeMethod, fakeParams, nil)
		rt.Verify()

		// bob (a signer) fails to cancel anne's transaction because bob didn't create it, nice try bob.
		rt.SetCaller(bob, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectAbort(exitcode.ErrForbidden, func() {
			proposalHashData := makeProposalHash(t, &multisig.Transaction{
				To:       chuck,
				Value:    sendValue,
				Method:   fakeMethod,
				Params:   fakeParams,
				Approved: []addr.Address{anne},
			})
			actor.cancel(rt, txnID, proposalHashData)
		})
		rt.Verify()

		// Transaction should remain after invalid cancel
		actor.assertTransactions(rt, multisig.Transaction{
			To:       chuck,
			Value:    sendValue,
			Method:   fakeMethod,
			Params:   fakeParams,
			Approved: []addr.Address{anne},
		})
	})

	t.Run("fail to cancel transaction when not signer", func(t *testing.T) {
		rt := builder.Build(t)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		// anne proposes a transaction
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, chuck, sendValue, fakeMethod, fakeParams, nil)
		rt.Verify()

		// richard (not a signer) fails to cancel anne's transaction because richard isn't a signer, go away richard.
		rt.SetCaller(richard, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectAbort(exitcode.ErrForbidden, func() {
			proposalHashData := makeProposalHash(t, &multisig.Transaction{
				To:       chuck,
				Value:    sendValue,
				Method:   fakeMethod,
				Params:   fakeParams,
				Approved: []addr.Address{anne},
			})
			actor.cancel(rt, txnID, proposalHashData)
		})
		rt.Verify()

		// Transaction should remain after invalid cancel
		actor.assertTransactions(rt, multisig.Transaction{
			To:       chuck,
			Value:    sendValue,
			Method:   fakeMethod,
			Params:   fakeParams,
			Approved: []addr.Address{anne},
		})
	})

	t.Run("fail to cancel a transaction that does not exist", func(t *testing.T) {
		rt := builder.Build(t)
		const dneTxnID = int64(1)

		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		// anne proposes a transaction ID: 0
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.proposeOK(rt, chuck, sendValue, fakeMethod, fakeParams, nil)
		rt.Verify()

		// anne fails to cancel a transaction that does not exists ID: 1 (dneTxnID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectAbort(exitcode.ErrNotFound, func() {
			proposalHashData := makeProposalHash(t, &multisig.Transaction{
				To:       chuck,
				Value:    sendValue,
				Method:   fakeMethod,
				Params:   fakeParams,
				Approved: []addr.Address{anne},
			})
			actor.cancel(rt, dneTxnID, proposalHashData)
		})
		rt.Verify()

		// Transaction should remain after invalid cancel
		actor.assertTransactions(rt, multisig.Transaction{
			To:       chuck,
			Value:    sendValue,
			Method:   fakeMethod,
			Params:   fakeParams,
			Approved: []addr.Address{anne},
		})
	})

	t.Run("transaction can ONLY be cancelled by a proposer who is still the signer", func(t *testing.T) {
		rt := builder.Build(t)
		const numApprovals = 3
		signers := []addr.Address{anne, bob, chuck}

		txnId := int64(0)
		actor.constructAndVerify(rt, numApprovals, noUnlockDuration, signers...)

		// anne proposes a transaction ID: 0
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		proposalHash := actor.proposeOK(rt, chuck, sendValue, fakeMethod, fakeParams, nil)

		// bob approves the transaction -> but he is the second approver and hence not the proposer
		rt.SetCaller(bob, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.approveOK(rt, txnId, proposalHash, nil)

		// remove anne as a signer - tx creator
		rt.SetCaller(receiver, builtin.MultisigActorCodeID)
		rt.ExpectValidateCallerAddr(receiver)
		actor.removeSigner(rt, anne, true)

		// anne fails to cancel a transaction - she is not a signer
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectAbort(exitcode.ErrForbidden, func() {
			actor.cancel(rt, txnID, proposalHash)
		})

		// bob fails to cancel the transaction -> he is not the proposer
		rt.SetCaller(bob, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectAbort(exitcode.ErrForbidden, func() {
			actor.cancel(rt, txnID, proposalHash)
		})

		// Transaction should remain after invalid cancel
		actor.assertTransactions(rt, multisig.Transaction{
			To:       chuck,
			Value:    sendValue,
			Method:   fakeMethod,
			Params:   fakeParams,
			Approved: []addr.Address{anne, bob},
		})

		// add anne as a signer again
		rt.SetCaller(receiver, builtin.MultisigActorCodeID)
		rt.ExpectValidateCallerAddr(receiver)
		actor.addSigner(rt, anne, true)

		// now anne can cancel the transaction
		rt.SetCaller(anne, builtin.AccountActorCodeID)
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		actor.cancel(rt, txnID, proposalHash)
		actor.assertTransactions(rt)
	})
}

type addSignerTestCase struct {
	desc string

	idAddrsMapping  map[addr.Address]addr.Address
	initialSigners   []addr.Address
	initialApprovals uint64

	addSigner addr.Address
	increase  bool

	expectSigners   []addr.Address
	expectApprovals uint64
	code            exitcode.ExitCode
}

func TestAddSigner(t *testing.T) {
	actor := msActorHarness{multisig.Actor{}, t}

	multisigWalletAdd := tutil.NewIDAddr(t, 100)
	anne := tutil.NewIDAddr(t, 101)
	bob := tutil.NewIDAddr(t, 102)
	chuck := tutil.NewIDAddr(t, 103)
	chuckNonId := tutil.NewBLSAddr(t,1)

	const noUnlockDuration = int64(0)

	testCases := []addSignerTestCase{
		{
			desc: "happy path add signer",

			initialSigners:   []addr.Address{anne, bob},
			initialApprovals: uint64(2),

			addSigner: chuck,
			increase:  false,

			expectSigners:   []addr.Address{anne, bob, chuck},
			expectApprovals: uint64(2),
			code:            exitcode.Ok,
		},
		{
			desc: "add signer and increase threshold",

			initialSigners:   []addr.Address{anne, bob},
			initialApprovals: uint64(2),

			addSigner: chuck,
			increase:  true,

			expectSigners:   []addr.Address{anne, bob, chuck},
			expectApprovals: uint64(3),
			code:            exitcode.Ok,
		},
		{
			desc: "fail to add signer than already exists",

			initialSigners:   []addr.Address{anne, bob, chuck},
			initialApprovals: uint64(3),

			addSigner: chuck,
			increase:  false,

			expectSigners:   []addr.Address{anne, bob, chuck},
			expectApprovals: uint64(3),
			code:            exitcode.ErrIllegalArgument,
		},
		{
			desc: "fail to add signer with ID address that already exists(even though we ONLY have the non ID address as an approver)",

			idAddrsMapping:   map[addr.Address]addr.Address{chuckNonId: chuck},
			initialSigners:   []addr.Address{anne, bob, chuckNonId},
			initialApprovals: uint64(3),

			addSigner: chuck,
			increase:  false,

			expectSigners:   []addr.Address{anne, bob, chuck},
			expectApprovals: uint64(3),
			code:            exitcode.ErrIllegalArgument,
		},
		{
			desc:             "fail to add signer with non-ID address that already exists(even though we ONLY have the ID address as an approver)",
			idAddrsMapping:   map[addr.Address]addr.Address{chuckNonId: chuck},
			initialSigners:   []addr.Address{anne, bob, chuck},
			initialApprovals: uint64(3),

			addSigner: chuckNonId,
			increase:  false,

			expectSigners:   []addr.Address{anne, bob, chuck},
			expectApprovals: uint64(3),
			code:            exitcode.ErrIllegalArgument,
		},
	}

	builder := mock.NewBuilder(context.Background(), multisigWalletAdd).WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			rt := builder.Build(t)

			actor.constructAndVerify(rt, tc.initialApprovals, noUnlockDuration, tc.initialSigners...)

			rt.SetCaller(multisigWalletAdd, builtin.AccountActorCodeID)
			rt.ExpectValidateCallerAddr(multisigWalletAdd)
			for src,target := range tc.idAddrsMapping {
				rt.AddIDAddress(src,target)
			}
			if tc.code != exitcode.Ok {
				rt.ExpectAbort(tc.code, func() {
					actor.addSigner(rt, tc.addSigner, tc.increase)
				})
			} else {
				actor.addSigner(rt, tc.addSigner, tc.increase)
				var st multisig.State
				rt.Readonly(&st)
				assert.Equal(t, tc.expectSigners, st.Signers)
				assert.Equal(t, tc.expectApprovals, st.NumApprovalsThreshold)
			}
			rt.Verify()
		})
	}
}

type removeSignerTestCase struct {
	desc string

	initialSigners   []addr.Address
	initialApprovals uint64

	removeSigner addr.Address
	decrease     bool

	expectSigners   []addr.Address
	expectApprovals uint64
	code            exitcode.ExitCode
}

func TestRemoveSigner(t *testing.T) {
	actor := msActorHarness{multisig.Actor{}, t}

	multisigWalletAdd := tutil.NewIDAddr(t, 100)
	anne := tutil.NewIDAddr(t, 101)
	bob := tutil.NewIDAddr(t, 102)
	chuck := tutil.NewIDAddr(t, 103)
	richard := tutil.NewIDAddr(t, 104)

	const noUnlockDuration = int64(0)

	testCases := []removeSignerTestCase{
		{
			desc: "happy path remove signer",

			initialSigners:   []addr.Address{anne, bob, chuck},
			initialApprovals: uint64(2),

			removeSigner: chuck,
			decrease:     false,

			expectSigners:   []addr.Address{anne, bob},
			expectApprovals: uint64(2),
			code:            exitcode.Ok,
		},
		{
			desc: "remove signer and decrease threshold",

			initialSigners:   []addr.Address{anne, bob, chuck},
			initialApprovals: uint64(2),

			removeSigner: chuck,
			decrease:     true,

			expectSigners:   []addr.Address{anne, bob},
			expectApprovals: uint64(1),
			code:            exitcode.Ok,
		},
		{
			desc: "fail remove signer if decrease set to false and number of signers below threshold",

			initialSigners:   []addr.Address{anne, bob, chuck},
			initialApprovals: uint64(3),

			removeSigner: chuck,
			decrease:     false,

			expectSigners:   []addr.Address{anne, bob},
			expectApprovals: uint64(2),
			code:            exitcode.ErrIllegalArgument,
		},
		{
			desc: "remove signer from single singer list",

			initialSigners:   []addr.Address{anne},
			initialApprovals: uint64(1),

			removeSigner: anne,
			decrease:     false,

			expectSigners:   nil,
			expectApprovals: uint64(1),
			code:            exitcode.ErrForbidden,
		},
		{
			desc: "fail to remove non-signer",

			initialSigners:   []addr.Address{anne, bob, chuck},
			initialApprovals: uint64(2),

			removeSigner: richard,
			decrease:     false,

			expectSigners:   []addr.Address{anne, bob, chuck},
			expectApprovals: uint64(2),
			code:            exitcode.ErrNotFound,
		},
	}

	builder := mock.NewBuilder(context.Background(), multisigWalletAdd).WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			rt := builder.Build(t)

			actor.constructAndVerify(rt, tc.initialApprovals, noUnlockDuration, tc.initialSigners...)

			rt.SetCaller(multisigWalletAdd, builtin.AccountActorCodeID)
			rt.ExpectValidateCallerAddr(multisigWalletAdd)
			if tc.code != exitcode.Ok {
				rt.ExpectAbort(tc.code, func() {
					actor.removeSigner(rt, tc.removeSigner, tc.decrease)
				})
			} else {
				actor.removeSigner(rt, tc.removeSigner, tc.decrease)
				var st multisig.State
				rt.Readonly(&st)
				assert.Equal(t, tc.expectSigners, st.Signers)
				assert.Equal(t, tc.expectApprovals, st.NumApprovalsThreshold)
			}
			rt.Verify()
		})
	}
}

type swapTestCase struct {
	initialSigner 	[]addr.Address
	idAddrMappings 	map[addr.Address]addr.Address
	desc   string
	to     addr.Address
	from   addr.Address
	expect []addr.Address
	code   exitcode.ExitCode
}

func TestSwapSigners(t *testing.T) {
	actor := msActorHarness{multisig.Actor{}, t}

	multisigWalletAdd := tutil.NewIDAddr(t, 100)
	anne := tutil.NewIDAddr(t, 101)
	bob := tutil.NewIDAddr(t, 102)
	bobNonId := tutil.NewBLSAddr(t,1)
	chuck := tutil.NewIDAddr(t, 103)
	darlene := tutil.NewIDAddr(t, 104)

	const noUnlockDuration = int64(0)
	const numApprovals = uint64(1)

	testCases := []swapTestCase{
		{
			desc:   "happy path signer swap",
			initialSigner : []addr.Address{anne, bob},
			to:     chuck,
			from:   bob,
			expect: []addr.Address{anne, chuck},
			code:   exitcode.Ok,
		},
		{
			desc:   "fail to swap when from signer not found",
			initialSigner : []addr.Address{anne, bob},
			to:     chuck,
			from:   darlene,
			expect: []addr.Address{anne, chuck},
			code:   exitcode.ErrNotFound,
		},
		{
			desc:   "fail to swap when to signer already present",
			initialSigner : []addr.Address{anne, bob},
			to:     bob,
			from:   anne,
			expect: []addr.Address{anne, chuck},
			code:   exitcode.ErrIllegalArgument,
		},
		{
			desc:   "fail to swap when to signer ID address already present(even though we have the non-ID address)",
			initialSigner : []addr.Address{anne, bobNonId},
			idAddrMappings: map[addr.Address]addr.Address{bobNonId:bob},
			to:     bob,
			from:   anne,
			expect: []addr.Address{anne, chuck},
			code:   exitcode.ErrIllegalArgument,
		},
		{
			desc:   "fail to swap when to signer non-ID address already present(even though we have the ID address)",
			initialSigner : []addr.Address{anne, bob},
			idAddrMappings: map[addr.Address]addr.Address{bobNonId:bob},
			to:     bobNonId,
			from:   anne,
			expect: []addr.Address{anne, chuck},
			code:   exitcode.ErrIllegalArgument,
		},
	}

	builder := mock.NewBuilder(context.Background(), multisigWalletAdd).WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			rt := builder.Build(t)

			actor.constructAndVerify(rt, numApprovals, noUnlockDuration, tc.initialSigner...)
			for src,target := range tc.idAddrMappings {
				rt.AddIDAddress(src, target)
			}

			rt.SetCaller(multisigWalletAdd, builtin.AccountActorCodeID)
			rt.ExpectValidateCallerAddr(multisigWalletAdd)
			if tc.code != exitcode.Ok {
				rt.ExpectAbort(tc.code, func() {
					actor.swapSigners(rt, tc.from, tc.to)
				})
			} else {
				actor.swapSigners(rt, tc.from, tc.to)
				var st multisig.State
				rt.Readonly(&st)
				assert.Equal(t, tc.expect, st.Signers)
			}
			rt.Verify()
		})
	}
}

type thresholdTestCase struct {
	desc             string
	initialThreshold uint64
	setThreshold     uint64
	code             exitcode.ExitCode
}

func TestChangeThreshold(t *testing.T) {
	actor := msActorHarness{multisig.Actor{}, t}

	multisigWalletAdd := tutil.NewIDAddr(t, 100)
	anne := tutil.NewIDAddr(t, 101)
	bob := tutil.NewIDAddr(t, 102)
	chuck := tutil.NewIDAddr(t, 103)

	const noUnlockDuration = int64(0)
	var initialSigner = []addr.Address{anne, bob, chuck}

	testCases := []thresholdTestCase{
		{
			desc:             "happy path decrease threshold",
			initialThreshold: 2,
			setThreshold:     1,
			code:             exitcode.Ok,
		},
		{
			desc:             "happy path simple increase threshold",
			initialThreshold: 2,
			setThreshold:     3,
			code:             exitcode.Ok,
		},
		{
			desc:             "fail to set threshold to zero",
			initialThreshold: 2,
			setThreshold:     0,
			code:             exitcode.ErrIllegalArgument,
		},
		{
			desc:             "fail to set threshold above number of signers",
			initialThreshold: 2,
			setThreshold:     uint64(len(initialSigner) + 1),
			code:             exitcode.ErrIllegalArgument,
		},
		// TODO missing test case that needs definition: https://github.com/filecoin-project/specs-actors/issues/71
		// what happens when threshold is reduced below the number of approvers an existing transaction already ha
	}

	builder := mock.NewBuilder(context.Background(), multisigWalletAdd).WithCaller(builtin.InitActorAddr, builtin.InitActorCodeID)
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			rt := builder.Build(t)

			actor.constructAndVerify(rt, tc.initialThreshold, noUnlockDuration, initialSigner...)

			rt.SetCaller(multisigWalletAdd, builtin.AccountActorCodeID)
			rt.ExpectValidateCallerAddr(multisigWalletAdd)
			if tc.code != exitcode.Ok {
				rt.ExpectAbort(tc.code, func() {
					actor.changeNumApprovalsThreshold(rt, tc.setThreshold)
				})
			} else {
				actor.changeNumApprovalsThreshold(rt, tc.setThreshold)
				var st multisig.State
				rt.Readonly(&st)
				assert.Equal(t, tc.setThreshold, st.NumApprovalsThreshold)
			}
			rt.Verify()
		})
	}
}

//
// Helper methods for calling multisig actor methods
//

type msActorHarness struct {
	a multisig.Actor
	t testing.TB
}

func (h *msActorHarness) constructAndVerify(rt *mock.Runtime, numApprovalsThresh uint64, unlockDuration int64, signers ...addr.Address) {
	constructParams := multisig.ConstructorParams{
		Signers:               signers,
		NumApprovalsThreshold: numApprovalsThresh,
		UnlockDuration:        abi.ChainEpoch(unlockDuration),
	}

	rt.ExpectValidateCallerAddr(builtin.InitActorAddr)
	ret := rt.Call(h.a.Constructor, &constructParams)
	assert.Nil(h.t, ret)
	rt.Verify()
}

func (h *msActorHarness) propose(rt *mock.Runtime, to addr.Address, value abi.TokenAmount, method abi.MethodNum, params []byte, out runtime.CBORUnmarshaler) exitcode.ExitCode {
	proposeParams := &multisig.ProposeParams{
		To:     to,
		Value:  value,
		Method: method,
		Params: params,
	}
	ret := rt.Call(h.a.Propose, proposeParams)
	rt.Verify()

	proposeReturn, ok := ret.(*multisig.ProposeReturn)
	if !ok {
		h.t.Fatalf("unexpected type returned from call to Propose")
	}
	// if the transaction was applied and a return value is expected deserialize it to the out parameter
	if proposeReturn.Applied {
		if out != nil {
			require.NoError(h.t, out.UnmarshalCBOR(bytes.NewReader(proposeReturn.Ret)))
		}
	}
	return proposeReturn.Code
}

// returns the proposal hash
func (h *msActorHarness) proposeOK(rt *mock.Runtime, to addr.Address, value abi.TokenAmount, method abi.MethodNum, params []byte, out runtime.CBORUnmarshaler) []byte {
	code := h.propose(rt, to, value, method, params, out)
	if code != exitcode.Ok {
		h.t.Fatalf("unexpected exitcode %d from propose", code)
	}

	proposalHashData, err := multisig.ComputeProposalHash(&multisig.Transaction{
		To:       to,
		Value:    value,
		Method:   method,
		Params:   params,
		Approved: []addr.Address{rt.Caller()},
	}, blake2b.Sum256)
	require.NoError(h.t, err)

	return proposalHashData
}

func (h *msActorHarness) approve(rt *mock.Runtime, txnID int64, proposalParams []byte, out runtime.CBORUnmarshaler) exitcode.ExitCode {
	approveParams := &multisig.TxnIDParams{ID: multisig.TxnID(txnID), ProposalHash: proposalParams}
	ret := rt.Call(h.a.Approve, approveParams)
	rt.Verify()
	approveReturn, ok := ret.(*multisig.ApproveReturn)
	if !ok {
		h.t.Fatalf("unexpected type returned from call to Approve")
	}
	// if the transaction was applied and a return value is expected deserialize it to the out parameter
	if approveReturn.Applied {
		if out != nil {
			require.NoError(h.t, out.UnmarshalCBOR(bytes.NewReader(approveReturn.Ret)))
		}
	}
	return approveReturn.Code
}

func (h *msActorHarness) approveOK(rt *mock.Runtime, txnID int64, proposalParams []byte, out runtime.CBORUnmarshaler) {
	code := h.approve(rt, txnID, proposalParams, out)
	if code != exitcode.Ok {
		h.t.Fatalf("unexpected exitcode %d from approve", code)
	}
}

func (h *msActorHarness) cancel(rt *mock.Runtime, txnID int64, proposalParams []byte) {
	cancelParams := &multisig.TxnIDParams{ID: multisig.TxnID(txnID), ProposalHash: proposalParams}
	rt.Call(h.a.Cancel, cancelParams)
	rt.Verify()
}

func (h *msActorHarness) addSigner(rt *mock.Runtime, signer addr.Address, increase bool) {
	addSignerParams := &multisig.AddSignerParams{
		Signer:   signer,
		Increase: increase,
	}
	rt.Call(h.a.AddSigner, addSignerParams)
	rt.Verify()
}

func (h *msActorHarness) removeSigner(rt *mock.Runtime, signer addr.Address, decrease bool) {
	rmSignerParams := &multisig.RemoveSignerParams{
		Signer:   signer,
		Decrease: decrease,
	}
	rt.Call(h.a.RemoveSigner, rmSignerParams)
	rt.Verify()
}

func (h *msActorHarness) swapSigners(rt *mock.Runtime, oldSigner, newSigner addr.Address) {
	swpParams := &multisig.SwapSignerParams{
		From: oldSigner,
		To:   newSigner,
	}
	rt.Call(h.a.SwapSigner, swpParams)
}

func (h *msActorHarness) changeNumApprovalsThreshold(rt *mock.Runtime, newThreshold uint64) {
	thrshParams := &multisig.ChangeNumApprovalsThresholdParams{NewThreshold: newThreshold}
	rt.Call(h.a.ChangeNumApprovalsThreshold, thrshParams)
}

func (h *msActorHarness) assertTransactions(rt *mock.Runtime, expected ...multisig.Transaction) {
	var st multisig.State
	rt.GetState(&st)

	txns, err := adt.AsMap(adt.AsStore(rt), st.PendingTxns)
	assert.NoError(h.t, err)
	keys, err := txns.CollectKeys()
	assert.NoError(h.t, err)

	require.Equal(h.t, len(expected), len(keys))
	for i, k := range keys {
		var actual multisig.Transaction
		found, err_ := txns.Get(asKey(k), &actual)
		require.NoError(h.t, err_)
		assert.True(h.t, found)
		assert.Equal(h.t, expected[i], actual)
	}
}

func makeProposalHash(t *testing.T, txn *multisig.Transaction) []byte {
	proposalHashData, err := multisig.ComputeProposalHash(txn, blake2b.Sum256)
	require.NoError(t, err)
	return proposalHashData
}

type key string

func (s key) Key() string {
	return string(s)
}

func asKey(in string) adt.Keyer {
	return key(in)
}
