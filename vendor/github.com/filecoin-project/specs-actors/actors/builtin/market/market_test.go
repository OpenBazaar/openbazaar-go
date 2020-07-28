package market_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/filecoin-project/specs-actors/support/mock"
	tutil "github.com/filecoin-project/specs-actors/support/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustCbor(o runtime.CBORMarshaler) []byte {
	buf := new(bytes.Buffer)
	if err := o.MarshalCBOR(buf); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

func TestExports(t *testing.T) {
	mock.CheckActorExports(t, market.Actor{})
}

func TestRemoveAllError(t *testing.T) {
	marketActor := tutil.NewIDAddr(t, 100)
	builder := mock.NewBuilder(context.Background(), marketActor)
	rt := builder.Build(t)
	store := adt.AsStore(rt)

	smm := market.MakeEmptySetMultimap(store)

	if err := smm.RemoveAll(42); err != nil {
		t.Fatalf("expected no error, got: %s", err)
	}
}

func TestMarketActor(t *testing.T) {
	marketActor := tutil.NewIDAddr(t, 100)
	owner := tutil.NewIDAddr(t, 101)
	provider := tutil.NewIDAddr(t, 102)
	worker := tutil.NewIDAddr(t, 103)
	client := tutil.NewIDAddr(t, 104)
	minerAddrs := &minerAddrs{owner, worker, provider}

	var st market.State

	t.Run("simple construction", func(t *testing.T) {
		actor := market.Actor{}
		receiver := tutil.NewIDAddr(t, 100)
		builder := mock.NewBuilder(context.Background(), receiver).
			WithCaller(builtin.SystemActorAddr, builtin.InitActorCodeID)

		rt := builder.Build(t)

		rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)

		ret := rt.Call(actor.Constructor, nil).(*adt.EmptyValue)
		assert.Nil(t, ret)
		rt.Verify()

		store := adt.AsStore(rt)

		emptyMap, err := adt.MakeEmptyMap(store).Root()
		assert.NoError(t, err)

		emptyArray, err := adt.MakeEmptyArray(store).Root()
		assert.NoError(t, err)

		emptyMultiMap, err := market.MakeEmptySetMultimap(store).Root()
		assert.NoError(t, err)

		var state market.State
		rt.GetState(&state)

		assert.Equal(t, emptyArray, state.Proposals)
		assert.Equal(t, emptyArray, state.States)
		assert.Equal(t, emptyMap, state.EscrowTable)
		assert.Equal(t, emptyMap, state.LockedTable)
		assert.Equal(t, abi.DealID(0), state.NextID)
		assert.Equal(t, emptyMultiMap, state.DealOpsByEpoch)
		assert.Equal(t, abi.ChainEpoch(-1), state.LastCron)
	})

	t.Run("AddBalance", func(t *testing.T) {
		t.Run("adds to provider escrow funds", func(t *testing.T) {
			testCases := []struct {
				delta int64
				total int64
			}{
				{10, 10},
				{20, 30},
				{40, 70},
			}

			// Test adding provider funds from both worker and owner address
			for _, callerAddr := range []address.Address{owner, worker} {
				rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)

				for _, tc := range testCases {
					rt.SetCaller(callerAddr, builtin.AccountActorCodeID)
					rt.SetReceived(abi.NewTokenAmount(tc.delta))
					actor.expectProviderControlAddressesAndValidateCaller(rt, provider, owner, worker)

					rt.Call(actor.AddBalance, &provider)

					rt.Verify()

					rt.GetState(&st)
					assert.Equal(t, abi.NewTokenAmount(tc.total), st.GetEscrowBalance(rt, provider))
				}
			}
		})

		t.Run("fails when called with negative value", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)

			rt.SetCaller(owner, builtin.AccountActorCodeID)
			rt.SetReceived(abi.NewTokenAmount(-1))

			rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
				rt.Call(actor.AddBalance, &provider)
			})

			rt.Verify()
		})

		t.Run("fails unless called by an account actor", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)

			rt.SetReceived(abi.NewTokenAmount(10))
			actor.expectProviderControlAddressesAndValidateCaller(rt, provider, owner, worker)

			rt.SetCaller(provider, builtin.StorageMinerActorCodeID)
			rt.ExpectAbort(exitcode.ErrForbidden, func() {
				rt.Call(actor.AddBalance, &provider)
			})

			rt.Verify()
		})

		t.Run("adds to non-provider escrow funds", func(t *testing.T) {
			testCases := []struct {
				delta int64
				total int64
			}{
				{10, 10},
				{20, 30},
				{40, 70},
			}

			// Test adding non-provider funds from both worker and client addresses
			for _, callerAddr := range []address.Address{client, worker} {
				rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)

				for _, tc := range testCases {
					rt.SetCaller(callerAddr, builtin.AccountActorCodeID)
					rt.SetReceived(abi.NewTokenAmount(tc.delta))
					rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)

					rt.Call(actor.AddBalance, &callerAddr)

					rt.Verify()

					rt.GetState(&st)
					assert.Equal(t, abi.NewTokenAmount(tc.total), st.GetEscrowBalance(rt, callerAddr))
				}
			}
		})
	})

	t.Run("WithdrawBalance", func(t *testing.T) {
		startEpoch := abi.ChainEpoch(10)
		endEpoch := abi.ChainEpoch(20)
		publishEpoch := abi.ChainEpoch(5)

		t.Run("fails with a negative withdraw amount", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)

			params := market.WithdrawBalanceParams{
				ProviderOrClientAddress: provider,
				Amount:                  abi.NewTokenAmount(-1),
			}

			rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
				rt.Call(actor.WithdrawBalance, &params)
			})

			rt.Verify()
		})

		t.Run("withdraws from provider escrow funds and sends to owner", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)

			actor.addProviderFunds(rt, abi.NewTokenAmount(20), minerAddrs)

			rt.GetState(&st)
			assert.Equal(t, abi.NewTokenAmount(20), st.GetEscrowBalance(rt, provider))

			// worker calls WithdrawBalance, balance is transferred to owner
			withdrawAmount := abi.NewTokenAmount(1)
			actor.withdrawProviderBalance(rt, withdrawAmount, withdrawAmount, minerAddrs)

			rt.GetState(&st)
			assert.Equal(t, abi.NewTokenAmount(19), st.GetEscrowBalance(rt, provider))
		})

		t.Run("withdraws from non-provider escrow funds", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)
			actor.addParticipantFunds(rt, client, abi.NewTokenAmount(20))

			rt.GetState(&st)
			assert.Equal(t, abi.NewTokenAmount(20), st.GetEscrowBalance(rt, client))

			withdrawAmount := abi.NewTokenAmount(1)
			actor.withdrawClientBalance(rt, client, withdrawAmount, withdrawAmount)

			rt.GetState(&st)
			assert.Equal(t, abi.NewTokenAmount(19), st.GetEscrowBalance(rt, client))
		})

		t.Run("client withdrawing more than escrow balance limits to available funds", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)
			actor.addParticipantFunds(rt, client, abi.NewTokenAmount(20))

			// withdraw amount greater than escrow balance
			withdrawAmount := abi.NewTokenAmount(25)
			expectedAmount := abi.NewTokenAmount(20)
			actor.withdrawClientBalance(rt, client, withdrawAmount, expectedAmount)

			rt.GetState(&st)
			assert.Equal(t, abi.NewTokenAmount(0), st.GetEscrowBalance(rt, client))
		})

		t.Run("worker withdrawing more than escrow balance limits to available funds", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)
			actor.addProviderFunds(rt, abi.NewTokenAmount(20), minerAddrs)

			rt.GetState(&st)
			assert.Equal(t, abi.NewTokenAmount(20), st.GetEscrowBalance(rt, provider))

			// withdraw amount greater than escrow balance
			withdrawAmount := abi.NewTokenAmount(25)
			actualWithdrawn := abi.NewTokenAmount(20)
			actor.withdrawProviderBalance(rt, withdrawAmount, actualWithdrawn, minerAddrs)

			rt.GetState(&st)
			assert.Equal(t, abi.NewTokenAmount(0), st.GetEscrowBalance(rt, provider))
		})

		t.Run("balance after withdrawal must ALWAYS be greater than or equal to locked amount", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)

			// create the deal to publish
			deal := actor.generateDealAndAddFunds(rt, client, minerAddrs, startEpoch, endEpoch)

			// publish the deal so that client AND provider collateral is locked
			rt.SetEpoch(publishEpoch)
			actor.publishDeals(rt, minerAddrs, deal)
			rt.GetState(&st)
			require.Equal(t, deal.ProviderCollateral, st.GetLockedBalance(rt, provider))
			require.Equal(t, deal.ClientBalanceRequirement(), st.GetLockedBalance(rt, client))

			withDrawAmt := abi.NewTokenAmount(1)
			withDrawableAmt := abi.NewTokenAmount(0)
			// client cannot withdraw any funds since all it's balance is locked
			actor.withdrawClientBalance(rt, client, withDrawAmt, withDrawableAmt)
			//  provider cannot withdraw any funds since all it's balance is locked
			actor.withdrawProviderBalance(rt, withDrawAmt, withDrawableAmt, minerAddrs)

			// add some more funds to the provider & ensure withdrawal is limited by the locked funds
			withDrawAmt = abi.NewTokenAmount(30)
			withDrawableAmt = abi.NewTokenAmount(25)
			actor.addProviderFunds(rt, withDrawableAmt, minerAddrs)
			actor.withdrawProviderBalance(rt, withDrawAmt, withDrawableAmt, minerAddrs)

			// add some more funds to the client & ensure withdrawal is limited by the locked funds
			actor.addParticipantFunds(rt, client, withDrawableAmt)
			actor.withdrawClientBalance(rt, client, withDrawAmt, withDrawableAmt)
		})

		t.Run("worker balance after withdrawal must account for slashed funds", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)

			// create the deal to publish
			deal := actor.generateDealAndAddFunds(rt, client, minerAddrs, startEpoch, endEpoch)

			// publish the deal
			rt.SetEpoch(publishEpoch)
			dealID := actor.publishDeals(rt, minerAddrs, deal)[0]

			// activate the deal
			actor.activateDeals(rt, []abi.DealID{dealID}, endEpoch+1, provider)
			st := actor.getDealState(rt, dealID)
			require.EqualValues(t, publishEpoch, st.SectorStartEpoch)

			// slash the deal
			rt.SetEpoch(publishEpoch + 1)
			actor.terminateDeals(rt, []abi.DealID{dealID}, provider)
			st = actor.getDealState(rt, dealID)
			require.EqualValues(t, publishEpoch+1, st.SlashEpoch)

			// provider cannot withdraw any funds since all it's balance is locked
			withDrawAmt := abi.NewTokenAmount(1)
			actualWithdrawn := abi.NewTokenAmount(0)
			actor.withdrawProviderBalance(rt, withDrawAmt, actualWithdrawn, minerAddrs)

			// add some more funds to the provider & ensure withdrawal is limited by the locked funds
			actor.addProviderFunds(rt, abi.NewTokenAmount(25), minerAddrs)
			withDrawAmt = abi.NewTokenAmount(30)
			actualWithdrawn = abi.NewTokenAmount(25)

			actor.withdrawProviderBalance(rt, withDrawAmt, actualWithdrawn, minerAddrs)
		})
	})
}

func TestPublishStorageDeals(t *testing.T) {
	marketActor := tutil.NewIDAddr(t, 100)
	owner := tutil.NewIDAddr(t, 101)
	provider := tutil.NewIDAddr(t, 102)
	worker := tutil.NewIDAddr(t, 103)
	client := tutil.NewIDAddr(t, 104)
	mAddr := &minerAddrs{owner, worker, provider}
	var st market.State

	t.Run("publish a deal after activating a previous deal which has a start epoch far in the future", func(t *testing.T) {
		startEpoch := abi.ChainEpoch(1000)
		endEpoch := abi.ChainEpoch(2000)
		publishEpoch := abi.ChainEpoch(1)

		rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)
		deal1 := actor.generateDealAndAddFunds(rt, client, mAddr, startEpoch, endEpoch)

		// publish the deal and activate it
		rt.SetEpoch(publishEpoch)
		deal1ID := actor.publishDeals(rt, mAddr, deal1)[0]
		actor.activateDeals(rt, []abi.DealID{deal1ID}, endEpoch, provider)
		st := actor.getDealState(rt, deal1ID)
		require.EqualValues(t, publishEpoch, st.SectorStartEpoch)

		// now publish a second deal and activate it
		deal2 := actor.generateDealAndAddFunds(rt, client, mAddr, startEpoch+1, endEpoch+1)
		rt.SetEpoch(publishEpoch + 1)
		deal2ID := actor.publishDeals(rt, mAddr, deal2)[0]
		actor.activateDeals(rt, []abi.DealID{deal2ID}, endEpoch+1, provider)
	})

	t.Run("publish multiple deals for different clients and ensure balances are correct", func(t *testing.T) {
		rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)
		client1 := tutil.NewIDAddr(t, 900)
		client2 := tutil.NewIDAddr(t, 901)
		client3 := tutil.NewIDAddr(t, 902)

		// generate first deal for
		deal1 := actor.generateDealAndAddFunds(rt, client1, mAddr, abi.ChainEpoch(42), abi.ChainEpoch(100))

		// generate second deal
		deal2 := actor.generateDealAndAddFunds(rt, client2, mAddr, abi.ChainEpoch(42), abi.ChainEpoch(100))

		// generate third deal
		deal3 := actor.generateDealAndAddFunds(rt, client3, mAddr, abi.ChainEpoch(42), abi.ChainEpoch(100))

		actor.publishDeals(rt, mAddr, deal1, deal2, deal3)

		// assert locked balance for all clients and provider
		providerLocked := big.Sum(deal1.ProviderCollateral, deal2.ProviderCollateral, deal3.ProviderCollateral)
		client1Locked := actor.getLockedBalance(rt, client1)
		client2Locked := actor.getLockedBalance(rt, client2)
		client3Locked := actor.getLockedBalance(rt, client3)
		require.EqualValues(t, deal1.ClientBalanceRequirement(), client1Locked)
		require.EqualValues(t, deal2.ClientBalanceRequirement(), client2Locked)
		require.EqualValues(t, deal3.ClientBalanceRequirement(), client3Locked)
		require.EqualValues(t, providerLocked, actor.getLockedBalance(rt, provider))

		// assert locked funds states
		rt.GetState(&st)
		totalClientCollateralLocked := big.Sum(deal3.ClientCollateral, deal1.ClientCollateral, deal2.ClientCollateral)
		require.EqualValues(t, totalClientCollateralLocked, st.TotalClientLockedCollateral)
		require.EqualValues(t, providerLocked, st.TotalProviderLockedCollateral)
		totalStorageFee := big.Sum(deal1.TotalStorageFee(), deal2.TotalStorageFee(), deal3.TotalStorageFee())
		require.EqualValues(t, totalStorageFee, st.TotalClientStorageFee)

		// publish two more deals for same clients with same provider
		deal4 := actor.generateDealAndAddFunds(rt, client3, mAddr, abi.ChainEpoch(1000), abi.ChainEpoch(10000))
		deal5 := actor.generateDealAndAddFunds(rt, client3, mAddr, abi.ChainEpoch(100), abi.ChainEpoch(1000))
		actor.publishDeals(rt, mAddr, deal4, deal5)

		// assert locked balances for clients and provider
		rt.GetState(&st)
		providerLocked = big.Sum(providerLocked, deal4.ProviderCollateral, deal5.ProviderCollateral)
		require.EqualValues(t, providerLocked, actor.getLockedBalance(rt, provider))

		client3LockedUpdated := actor.getLockedBalance(rt, client3)
		require.EqualValues(t, big.Sum(client3Locked, deal4.ClientBalanceRequirement(), deal5.ClientBalanceRequirement()), client3LockedUpdated)

		client1Locked = actor.getLockedBalance(rt, client1)
		client2Locked = actor.getLockedBalance(rt, client2)
		require.EqualValues(t, deal1.ClientBalanceRequirement(), client1Locked)
		require.EqualValues(t, deal2.ClientBalanceRequirement(), client2Locked)

		// assert locked funds states
		totalClientCollateralLocked = big.Sum(totalClientCollateralLocked, deal4.ClientCollateral, deal5.ClientCollateral)
		require.EqualValues(t, totalClientCollateralLocked, st.TotalClientLockedCollateral)
		require.EqualValues(t, providerLocked, st.TotalProviderLockedCollateral)

		totalStorageFee = big.Sum(totalStorageFee, deal4.TotalStorageFee(), deal5.TotalStorageFee())
		require.EqualValues(t, totalStorageFee, st.TotalClientStorageFee)

		// PUBLISH DEALS with a different provider
		provider2 := tutil.NewIDAddr(t, 109)
		miner := &minerAddrs{owner, worker, provider2}

		// generate first deal for second provider
		deal6 := actor.generateDealAndAddFunds(rt, client1, miner, abi.ChainEpoch(20), abi.ChainEpoch(50))

		// generate second deal for second provider
		deal7 := actor.generateDealAndAddFunds(rt, client1, miner, abi.ChainEpoch(25), abi.ChainEpoch(60))

		// publish both the deals for the second provider
		actor.publishDeals(rt, miner, deal6, deal7)

		// assertions
		rt.GetState(&st)
		provider2Locked := big.Add(deal6.ProviderCollateral, deal7.ProviderCollateral)
		require.EqualValues(t, provider2Locked, actor.getLockedBalance(rt, provider2))
		client1LockedUpdated := actor.getLockedBalance(rt, client1)
		require.EqualValues(t, big.Add(deal7.ClientBalanceRequirement(), big.Add(client1Locked, deal6.ClientBalanceRequirement())), client1LockedUpdated)

		// assert first provider's balance as well
		require.EqualValues(t, providerLocked, actor.getLockedBalance(rt, provider))

		totalClientCollateralLocked = big.Add(totalClientCollateralLocked, big.Add(deal6.ClientCollateral, deal7.ClientCollateral))
		require.EqualValues(t, totalClientCollateralLocked, st.TotalClientLockedCollateral)
		require.EqualValues(t, big.Add(providerLocked, provider2Locked), st.TotalProviderLockedCollateral)
		totalStorageFee = big.Add(totalStorageFee, big.Add(deal6.TotalStorageFee(), deal7.TotalStorageFee()))
		require.EqualValues(t, totalStorageFee, st.TotalClientStorageFee)
	})
}

func TestPublishStorageDealsFailures(t *testing.T) {
	marketActor := tutil.NewIDAddr(t, 100)
	owner := tutil.NewIDAddr(t, 101)
	provider := tutil.NewIDAddr(t, 102)
	worker := tutil.NewIDAddr(t, 103)
	client := tutil.NewIDAddr(t, 104)
	mAddrs := &minerAddrs{owner, worker, provider}

	currentEpoch := abi.ChainEpoch(5)
	startEpoch := abi.ChainEpoch(10)
	endEpoch := abi.ChainEpoch(20)

	// simple failures because of invalid deal params
	{
		tcs := map[string]struct {
			setup                      func(*mock.Runtime, *marketActorTestHarness, *market.DealProposal)
			exitCode                   exitcode.ExitCode
			signatureVerificationError error
		}{
			"deal end after deal start": {
				setup: func(_ *mock.Runtime, _ *marketActorTestHarness, d *market.DealProposal) {
					d.StartEpoch = 10
					d.EndEpoch = 9
				},
				exitCode: exitcode.ErrIllegalArgument,
			},
			"current epoch greater than start epoch": {
				setup: func(_ *mock.Runtime, _ *marketActorTestHarness, d *market.DealProposal) {
					d.StartEpoch = currentEpoch - 1
				},
				exitCode: exitcode.ErrIllegalArgument,
			},
			"deal duration greater than max deal duration": {
				setup: func(_ *mock.Runtime, _ *marketActorTestHarness, d *market.DealProposal) {
					d.StartEpoch = abi.ChainEpoch(10)
					d.EndEpoch = d.StartEpoch + (1 * builtin.EpochsInYear) + 1
				},
				exitCode: exitcode.ErrIllegalArgument,
			},
			"negative price per epoch": {
				setup: func(_ *mock.Runtime, _ *marketActorTestHarness, d *market.DealProposal) {
					d.StoragePricePerEpoch = abi.NewTokenAmount(-1)
				},
				exitCode: exitcode.ErrIllegalArgument,
			},
			"price per epoch greater than total filecoin": {
				setup: func(_ *mock.Runtime, _ *marketActorTestHarness, d *market.DealProposal) {
					d.StoragePricePerEpoch = big.Add(abi.TotalFilecoin, big.NewInt(1))
				},
				exitCode: exitcode.ErrIllegalArgument,
			},
			"negative provider collateral": {
				setup: func(_ *mock.Runtime, _ *marketActorTestHarness, d *market.DealProposal) {
					d.ProviderCollateral = big.NewInt(-1)
				},
				exitCode: exitcode.ErrIllegalArgument,
			},
			"provider collateral greater than max collateral": {
				setup: func(_ *mock.Runtime, _ *marketActorTestHarness, d *market.DealProposal) {
					d.ProviderCollateral = big.Add(abi.TotalFilecoin, big.NewInt(1))
				},
				exitCode: exitcode.ErrIllegalArgument,
			},
			"negative client collateral": {
				setup: func(_ *mock.Runtime, _ *marketActorTestHarness, d *market.DealProposal) {
					d.ClientCollateral = big.NewInt(-1)
				},
				exitCode: exitcode.ErrIllegalArgument,
			},
			"client collateral greater than max collateral": {
				setup: func(_ *mock.Runtime, _ *marketActorTestHarness, d *market.DealProposal) {
					d.ClientCollateral = big.Add(abi.TotalFilecoin, big.NewInt(1))
				},
				exitCode: exitcode.ErrIllegalArgument,
			},
			"client does not have enough balance for collateral": {
				setup: func(rt *mock.Runtime, a *marketActorTestHarness, d *market.DealProposal) {
					a.addParticipantFunds(rt, client, big.Sub(d.ClientBalanceRequirement(), big.NewInt(1)))
					a.addProviderFunds(rt, d.ProviderCollateral, mAddrs)
				},
				exitCode: exitcode.ErrInsufficientFunds,
			},
			"provider does not have enough balance for collateral": {
				setup: func(rt *mock.Runtime, a *marketActorTestHarness, d *market.DealProposal) {
					a.addParticipantFunds(rt, client, d.ClientBalanceRequirement())
					a.addProviderFunds(rt, big.Sub(d.ProviderCollateral, big.NewInt(1)), mAddrs)
				},
				exitCode: exitcode.ErrInsufficientFunds,
			},
			"unable to resolve client address": {
				setup: func(_ *mock.Runtime, a *marketActorTestHarness, d *market.DealProposal) {
					d.Client = tutil.NewBLSAddr(t, 1)
				},
				exitCode: exitcode.ErrNotFound,
			},
			"signature is invalid": {
				setup: func(_ *mock.Runtime, a *marketActorTestHarness, d *market.DealProposal) {

				},
				exitCode:                   exitcode.ErrIllegalArgument,
				signatureVerificationError: errors.New("error"),
			},
			"no entry for client in locked  balance table": {
				setup: func(rt *mock.Runtime, a *marketActorTestHarness, d *market.DealProposal) {
					a.addProviderFunds(rt, d.ProviderCollateral, mAddrs)
				},
				exitCode: exitcode.ErrInsufficientFunds,
			},
			"no entry for provider in locked  balance table": {
				setup: func(rt *mock.Runtime, a *marketActorTestHarness, d *market.DealProposal) {
					a.addParticipantFunds(rt, client, d.ClientBalanceRequirement())
				},
				exitCode: exitcode.ErrInsufficientFunds,
			},
		}

		for name, tc := range tcs {
			t.Run(name, func(t *testing.T) {
				rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)
				dealProposal := generateDealProposal(client, provider, startEpoch, endEpoch)
				rt.SetEpoch(currentEpoch)
				tc.setup(rt, actor, &dealProposal)
				params := mkPublishStorageParams(dealProposal)

				rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
				rt.ExpectSend(provider, builtin.MethodsMiner.ControlAddresses, nil, abi.NewTokenAmount(0), &miner.GetControlAddressesReturn{Worker: worker, Owner: owner}, 0)
				rt.SetCaller(worker, builtin.AccountActorCodeID)
				rt.ExpectVerifySignature(crypto.Signature{}, dealProposal.Client, mustCbor(&dealProposal), tc.signatureVerificationError)
				rt.ExpectAbort(tc.exitCode, func() {
					rt.Call(actor.PublishStorageDeals, params)
				})

				rt.Verify()
			})
		}
	}

	// fails when client or provider has some funds but not enough to cover a deal
	{
		t.Run("fail when client has some funds but not enough for a deal", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)

			//
			actor.addParticipantFunds(rt, client, abi.NewTokenAmount(100))
			deal1 := generateDealProposal(client, provider, abi.ChainEpoch(42), abi.ChainEpoch(100))
			actor.addProviderFunds(rt, deal1.ProviderCollateral, mAddrs)
			params := mkPublishStorageParams(deal1)

			rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
			rt.ExpectSend(provider, builtin.MethodsMiner.ControlAddresses, nil, abi.NewTokenAmount(0), &miner.GetControlAddressesReturn{Worker: worker, Owner: owner}, 0)
			rt.SetCaller(worker, builtin.AccountActorCodeID)
			rt.ExpectVerifySignature(crypto.Signature{}, deal1.Client, mustCbor(&deal1), nil)
			rt.ExpectAbort(exitcode.ErrInsufficientFunds, func() {
				rt.Call(actor.PublishStorageDeals, params)
			})

			rt.Verify()
		})

		t.Run("fail when provider has some funds but not enough for a deal", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)

			actor.addProviderFunds(rt, abi.NewTokenAmount(1), mAddrs)
			deal1 := generateDealProposal(client, provider, abi.ChainEpoch(42), abi.ChainEpoch(100))
			actor.addParticipantFunds(rt, client, deal1.ClientBalanceRequirement())

			params := mkPublishStorageParams(deal1)

			rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
			rt.ExpectSend(provider, builtin.MethodsMiner.ControlAddresses, nil, abi.NewTokenAmount(0), &miner.GetControlAddressesReturn{Worker: worker, Owner: owner}, 0)
			rt.SetCaller(worker, builtin.AccountActorCodeID)
			rt.ExpectVerifySignature(crypto.Signature{}, deal1.Client, mustCbor(&deal1), nil)
			rt.ExpectAbort(exitcode.ErrInsufficientFunds, func() {
				rt.Call(actor.PublishStorageDeals, params)
			})

			rt.Verify()
		})
	}

	// fail when deals have different providers
	{
		t.Run("fail when deals have different providers", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)
			deal1 := actor.generateDealAndAddFunds(rt, client, mAddrs, abi.ChainEpoch(42), abi.ChainEpoch(100))
			m2 := &minerAddrs{owner, worker, tutil.NewIDAddr(t, 1000)}

			deal2 := actor.generateDealAndAddFunds(rt, client, m2, abi.ChainEpoch(1), abi.ChainEpoch(5))

			params := mkPublishStorageParams(deal1, deal2)

			rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
			rt.ExpectSend(provider, builtin.MethodsMiner.ControlAddresses, nil, abi.NewTokenAmount(0), &miner.GetControlAddressesReturn{Worker: worker, Owner: owner}, 0)
			rt.SetCaller(worker, builtin.AccountActorCodeID)
			rt.ExpectVerifySignature(crypto.Signature{}, deal1.Client, mustCbor(&deal1), nil)
			rt.ExpectVerifySignature(crypto.Signature{}, deal2.Client, mustCbor(&deal2), nil)
			rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
				rt.Call(actor.PublishStorageDeals, params)
			})

			rt.Verify()
		})

		//  failures because of incorrect call params
		t.Run("fail when caller is not of signable type", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)
			params := mkPublishStorageParams(generateDealProposal(client, provider, abi.ChainEpoch(1), abi.ChainEpoch(5)))
			w := tutil.NewIDAddr(t, 1000)
			rt.SetCaller(w, builtin.StorageMinerActorCodeID)
			rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
			rt.ExpectAbort(exitcode.ErrForbidden, func() {
				rt.Call(actor.PublishStorageDeals, params)
			})
		})

		t.Run("fail when no deals in params", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)
			params := mkPublishStorageParams()
			rt.SetCaller(worker, builtin.AccountActorCodeID)
			rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
			rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
				rt.Call(actor.PublishStorageDeals, params)
			})
		})

		t.Run("fail to resolve provider address", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)
			deal := generateDealProposal(client, provider, abi.ChainEpoch(1), abi.ChainEpoch(5))
			deal.Provider = tutil.NewBLSAddr(t, 100)

			params := mkPublishStorageParams(deal)
			rt.SetCaller(worker, builtin.AccountActorCodeID)
			rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
			rt.ExpectAbort(exitcode.ErrNotFound, func() {
				rt.Call(actor.PublishStorageDeals, params)
			})
		})

		t.Run("caller is not the same as the worker address for miner", func(t *testing.T) {
			rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)
			deal := generateDealProposal(client, provider, abi.ChainEpoch(1), abi.ChainEpoch(5))
			params := mkPublishStorageParams(deal)
			rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
			rt.ExpectSend(provider, builtin.MethodsMiner.ControlAddresses, nil, abi.NewTokenAmount(0), &miner.GetControlAddressesReturn{Worker: tutil.NewIDAddr(t, 999), Owner: owner}, 0)
			rt.SetCaller(worker, builtin.AccountActorCodeID)
			rt.ExpectAbort(exitcode.ErrForbidden, func() {
				rt.Call(actor.PublishStorageDeals, params)
			})

			rt.Verify()
		})
	}
}

func TestMarketActorDeals(t *testing.T) {
	marketActor := tutil.NewIDAddr(t, 100)
	owner := tutil.NewIDAddr(t, 101)
	provider := tutil.NewIDAddr(t, 102)
	worker := tutil.NewIDAddr(t, 103)
	client := tutil.NewIDAddr(t, 104)
	minerAddrs := &minerAddrs{owner, worker, provider}

	var st market.State

	// Test adding provider funds from both worker and owner address
	rt, actor := basicMarketSetup(t, marketActor, owner, provider, worker, client)
	actor.addProviderFunds(rt, abi.NewTokenAmount(10000), minerAddrs)
	rt.GetState(&st)
	assert.Equal(t, abi.NewTokenAmount(10000), st.GetEscrowBalance(rt, provider))

	actor.addParticipantFunds(rt, client, abi.NewTokenAmount(10000))

	dealProposal := generateDealProposal(client, provider, abi.ChainEpoch(1), abi.ChainEpoch(5))
	params := &market.PublishStorageDealsParams{Deals: []market.ClientDealProposal{market.ClientDealProposal{Proposal: dealProposal}}}

	// First attempt at publishing the deal should work
	{
		actor.publishDeals(rt, minerAddrs, dealProposal)
	}

	// Second attempt at publishing the same deal should fail
	{
		rt.ExpectValidateCallerType(builtin.AccountActorCodeID, builtin.MultisigActorCodeID)
		rt.ExpectSend(provider, builtin.MethodsMiner.ControlAddresses, nil, abi.NewTokenAmount(0), &miner.GetControlAddressesReturn{Worker: worker, Owner: owner}, 0)

		rt.ExpectVerifySignature(crypto.Signature{}, client, mustCbor(&params.Deals[0].Proposal), nil)
		rt.SetCaller(worker, builtin.AccountActorCodeID)
		rt.ExpectAbort(exitcode.ErrIllegalArgument, func() {
			rt.Call(actor.PublishStorageDeals, params)
		})

		rt.Verify()
	}

	dealProposal.Label = "foo"

	// Same deal with a different label should work
	{
		actor.publishDeals(rt, minerAddrs, dealProposal)
	}
}

type marketActorTestHarness struct {
	market.Actor
	t testing.TB
}

func (h *marketActorTestHarness) constructAndVerify(rt *mock.Runtime) {
	rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
	ret := rt.Call(h.Constructor, nil)
	assert.Nil(h.t, ret)
	rt.Verify()
}

type minerAddrs struct {
	owner    address.Address
	worker   address.Address
	provider address.Address
}

// addProviderFunds is a helper method to setup provider market funds
func (h *marketActorTestHarness) addProviderFunds(rt *mock.Runtime, amount abi.TokenAmount, minerAddrs *minerAddrs) {
	rt.SetReceived(amount)
	rt.SetAddressActorType(minerAddrs.provider, builtin.StorageMinerActorCodeID)
	rt.SetCaller(minerAddrs.owner, builtin.AccountActorCodeID)
	h.expectProviderControlAddressesAndValidateCaller(rt, minerAddrs.provider, minerAddrs.owner, minerAddrs.worker)

	rt.Call(h.AddBalance, &minerAddrs.provider)

	rt.Verify()

	rt.SetBalance(big.Add(rt.Balance(), amount))
}

// addParticipantFunds is a helper method to setup non-provider storage market participant funds
func (h *marketActorTestHarness) addParticipantFunds(rt *mock.Runtime, addr address.Address, amount abi.TokenAmount) {
	rt.SetReceived(amount)
	rt.SetCaller(addr, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)

	rt.Call(h.AddBalance, &addr)

	rt.Verify()

	rt.SetBalance(big.Add(rt.Balance(), amount))
}

func (h *marketActorTestHarness) expectProviderControlAddressesAndValidateCaller(rt *mock.Runtime, provider address.Address, owner address.Address, worker address.Address) {
	rt.ExpectValidateCallerAddr(owner, worker)

	expectRet := &miner.GetControlAddressesReturn{Owner: owner, Worker: worker}

	rt.ExpectSend(
		provider,
		builtin.MethodsMiner.ControlAddresses,
		nil,
		big.Zero(),
		expectRet,
		exitcode.Ok,
	)
}

func (h *marketActorTestHarness) withdrawProviderBalance(rt *mock.Runtime, withDrawAmt, expectedSend abi.TokenAmount, miner *minerAddrs) {
	rt.SetCaller(miner.worker, builtin.AccountActorCodeID)
	h.expectProviderControlAddressesAndValidateCaller(rt, miner.provider, miner.owner, miner.worker)

	params := market.WithdrawBalanceParams{
		ProviderOrClientAddress: miner.provider,
		Amount:                  withDrawAmt,
	}

	rt.ExpectSend(miner.owner, builtin.MethodSend, nil, expectedSend, nil, exitcode.Ok)
	rt.Call(h.WithdrawBalance, &params)
	rt.Verify()
}

func (h *marketActorTestHarness) withdrawClientBalance(rt *mock.Runtime, client address.Address, withDrawAmt, expectedSend abi.TokenAmount) {
	rt.SetCaller(client, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
	rt.ExpectSend(client, builtin.MethodSend, nil, expectedSend, nil, exitcode.Ok)

	params := market.WithdrawBalanceParams{
		ProviderOrClientAddress: client,
		Amount:                  withDrawAmt,
	}

	rt.Call(h.WithdrawBalance, &params)
	rt.Verify()
}

func (h *marketActorTestHarness) publishDeals(rt *mock.Runtime, minerAddrs *minerAddrs, deals ...market.DealProposal) []abi.DealID {
	rt.SetCaller(minerAddrs.worker, builtin.AccountActorCodeID)
	rt.ExpectValidateCallerType(builtin.CallerTypesSignable...)
	rt.ExpectSend(
		minerAddrs.provider,
		builtin.MethodsMiner.ControlAddresses,
		nil,
		big.Zero(),
		&miner.GetControlAddressesReturn{Owner: minerAddrs.owner, Worker: minerAddrs.worker},
		exitcode.Ok,
	)

	var params market.PublishStorageDealsParams

	for _, deal := range deals {
		//  create a client proposal with a valid signature
		buf := bytes.Buffer{}
		require.NoError(h.t, deal.MarshalCBOR(&buf), "failed to marshal deal proposal")
		sig := crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte("does not matter")}
		clientProposal := market.ClientDealProposal{deal, sig}
		params.Deals = append(params.Deals, clientProposal)

		// expect a call to verify the above signature
		rt.ExpectVerifySignature(sig, deal.Client, buf.Bytes(), nil)
	}

	ret := rt.Call(h.PublishStorageDeals, &params)
	rt.Verify()

	resp, ok := ret.(*market.PublishStorageDealsReturn)
	require.True(h.t, ok, "unexpected type returned from call to PublishStorageDeals")
	require.Len(h.t, resp.IDs, len(deals))

	// assert state after publishing the deals
	dealIds := resp.IDs
	for i, deaId := range dealIds {
		expected := deals[i]
		p := h.getDealProposal(rt, deaId)

		require.Equal(h.t, expected.StartEpoch, p.StartEpoch)
		require.Equal(h.t, expected.EndEpoch, p.EndEpoch)
		require.Equal(h.t, expected.PieceCID, p.PieceCID)
		require.Equal(h.t, expected.PieceSize, p.PieceSize)
		require.Equal(h.t, expected.Client, p.Client)
		require.Equal(h.t, expected.Provider, p.Provider)
		require.Equal(h.t, expected.Label, p.Label)
		require.Equal(h.t, expected.VerifiedDeal, p.VerifiedDeal)
		require.Equal(h.t, expected.StoragePricePerEpoch, p.StoragePricePerEpoch)
		require.Equal(h.t, expected.ClientCollateral, p.ClientCollateral)
		require.Equal(h.t, expected.ProviderCollateral, p.ProviderCollateral)
	}

	return resp.IDs
}

func (h *marketActorTestHarness) activateDeals(rt *mock.Runtime, dealIDs []abi.DealID, sectorExpiry abi.ChainEpoch, minerAddr address.Address) {
	rt.SetCaller(minerAddr, builtin.StorageMinerActorCodeID)
	rt.ExpectValidateCallerType(builtin.StorageMinerActorCodeID)

	params := &market.ActivateDealsParams{DealIDs: dealIDs, SectorExpiry: sectorExpiry}

	ret := rt.Call(h.ActivateDeals, params)
	rt.Verify()

	require.Nil(h.t, ret)
}

func (h *marketActorTestHarness) getDealProposal(rt *mock.Runtime, dealID abi.DealID) *market.DealProposal {
	var st market.State
	rt.GetState(&st)

	deals, err := market.AsDealProposalArray(adt.AsStore(rt), st.Proposals)
	require.NoError(h.t, err)

	d, found, err := deals.Get(dealID)
	require.NoError(h.t, err)
	require.True(h.t, found)
	require.NotNil(h.t, d)

	return d
}

func (h *marketActorTestHarness) getLockedBalance(rt *mock.Runtime, addr address.Address) abi.TokenAmount {
	var st market.State
	rt.GetState(&st)

	return st.GetLockedBalance(rt, addr)
}

func (h *marketActorTestHarness) getDealState(rt *mock.Runtime, dealID abi.DealID) *market.DealState {
	var st market.State
	rt.GetState(&st)

	states, err := market.AsDealStateArray(adt.AsStore(rt), st.States)
	require.NoError(h.t, err)

	s, found, err := states.Get(dealID)
	require.NoError(h.t, err)
	require.True(h.t, found)
	require.NotNil(h.t, s)

	return s
}

func (h *marketActorTestHarness) terminateDeals(rt *mock.Runtime, dealIDs []abi.DealID, minerAddr address.Address) {
	rt.SetCaller(minerAddr, builtin.StorageMinerActorCodeID)
	rt.ExpectValidateCallerType(builtin.StorageMinerActorCodeID)

	params := &market.OnMinerSectorsTerminateParams{DealIDs: dealIDs}

	ret := rt.Call(h.OnMinerSectorsTerminate, params)
	rt.Verify()

	require.Nil(h.t, ret)
}

func (h *marketActorTestHarness) generateDealAndAddFunds(rt *mock.Runtime, client address.Address, minerAddrs *minerAddrs,
	startEpoch, endEpoch abi.ChainEpoch) market.DealProposal {
	deal4 := generateDealProposal(client, minerAddrs.provider, startEpoch, endEpoch)
	h.addProviderFunds(rt, deal4.ProviderCollateral, minerAddrs)
	h.addParticipantFunds(rt, client, deal4.ClientBalanceRequirement())
	return deal4
}

func generateDealProposal(client, provider address.Address, startEpoch, endEpoch abi.ChainEpoch) market.DealProposal {
	pieceCid := tutil.MakeCID("1")
	pieceSize := abi.PaddedPieceSize(2048)
	storagePerEpoch := big.NewInt(10)
	clientCollateral := big.NewInt(10)
	providerCollateral := big.NewInt(10)

	return market.DealProposal{pieceCid, pieceSize, false, client, provider, "label", startEpoch,
		endEpoch, storagePerEpoch, providerCollateral, clientCollateral}
}

func basicMarketSetup(t *testing.T, ma, owner, provider, worker, client address.Address) (*mock.Runtime, *marketActorTestHarness) {
	builder := mock.NewBuilder(context.Background(), ma).
		WithCaller(builtin.SystemActorAddr, builtin.InitActorCodeID).
		WithActorType(owner, builtin.AccountActorCodeID).
		WithActorType(worker, builtin.AccountActorCodeID).
		WithActorType(provider, builtin.StorageMinerActorCodeID).
		WithActorType(client, builtin.AccountActorCodeID)

	rt := builder.Build(t)

	actor := marketActorTestHarness{t: t}
	actor.constructAndVerify(rt)

	return rt, &actor
}

func mkPublishStorageParams(proposals ...market.DealProposal) *market.PublishStorageDealsParams {
	m := &market.PublishStorageDealsParams{}
	for _, p := range proposals {
		m.Deals = append(m.Deals, market.ClientDealProposal{Proposal: p})
	}
	return m
}
