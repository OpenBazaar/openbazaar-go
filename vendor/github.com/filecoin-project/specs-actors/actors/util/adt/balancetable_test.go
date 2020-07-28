package adt_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/filecoin-project/specs-actors/support/mock"
	tutil "github.com/filecoin-project/specs-actors/support/testing"
)

func TestBalanceTable(t *testing.T) {
	t.Run("AddCreate adds or creates", func(t *testing.T) {
		addr := tutil.NewIDAddr(t, 100)
		rt := mock.NewBuilder(context.Background(), address.Undef).Build(t)
		store := adt.AsStore(rt)
		emptyMap := adt.MakeEmptyMap(store)

		bt, err := adt.AsBalanceTable(store, tutil.MustRoot(t, emptyMap))
		assert.NoError(t, err)

		has, err := bt.Has(addr)
		assert.NoError(t, err)
		assert.False(t, has)

		err = bt.AddCreate(addr, abi.NewTokenAmount(10))
		assert.NoError(t, err)

		amount, err := bt.Get(addr)
		assert.NoError(t, err)
		assert.Equal(t, abi.NewTokenAmount(10), amount)

		err = bt.AddCreate(addr, abi.NewTokenAmount(20))
		assert.NoError(t, err)

		amount, err = bt.Get(addr)
		assert.NoError(t, err)
		assert.Equal(t, abi.NewTokenAmount(30), amount)
	})

	t.Run("Total returns total amount tracked", func(t *testing.T) {
		addr1 := tutil.NewIDAddr(t, 100)
		addr2 := tutil.NewIDAddr(t, 101)

		rt := mock.NewBuilder(context.Background(), address.Undef).Build(t)
		store := adt.AsStore(rt)
		emptyMap := adt.MakeEmptyMap(store)

		bt, err := adt.AsBalanceTable(store, tutil.MustRoot(t, emptyMap))
		assert.NoError(t, err)
		total, err := bt.Total()
		assert.NoError(t, err)
		assert.Equal(t, big.Zero(), total)

		testCases := []struct {
			amount int64
			addr   address.Address
			total  int64
		}{
			{10, addr1, 10},
			{20, addr1, 30},
			{40, addr2, 70},
			{50, addr2, 120},
		}

		for _, tc := range testCases {
			err = bt.AddCreate(tc.addr, abi.NewTokenAmount(tc.amount))
			assert.NoError(t, err)

			total, err = bt.Total()
			assert.NoError(t, err)
			assert.Equal(t, abi.NewTokenAmount(tc.total), total)
		}
	})
}
