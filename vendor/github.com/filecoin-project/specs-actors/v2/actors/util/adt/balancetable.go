package adt

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	cid "github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
)

// A specialization of a map of addresses to (positive) token amounts.
// Absent keys implicitly have a balance of zero.
type BalanceTable Map

// Interprets a store as balance table with root `r`.
func AsBalanceTable(s Store, r cid.Cid) (*BalanceTable, error) {
	m, err := AsMap(s, r)
	if err != nil {
		return nil, err
	}

	return &BalanceTable{
		root:  m.root,
		store: s,
	}, nil
}

// Returns the root cid of underlying HAMT.
func (t *BalanceTable) Root() (cid.Cid, error) {
	return (*Map)(t).Root()
}

// Gets the balance for a key, which is zero if they key has never been added to.
func (t *BalanceTable) Get(key addr.Address) (abi.TokenAmount, error) {
	var value abi.TokenAmount
	found, err := (*Map)(t).Get(abi.AddrKey(key), &value)
	if !found || err != nil {
		value = big.Zero()
	}

	return value, err
}

// Adds an amount to a balance, requiring the resulting balance to be non-negative.
func (t *BalanceTable) Add(key addr.Address, value abi.TokenAmount) error {
	prev, err := t.Get(key)
	if err != nil {
		return err
	}
	sum := big.Add(prev, value)
	sign := sum.Sign()
	if sign < 0 {
		return xerrors.Errorf("adding %v to balance %v would give negative: %v", value, prev, sum)
	} else if sign == 0 && !prev.IsZero() {
		return (*Map)(t).Delete(abi.AddrKey(key))
	}
	return (*Map)(t).Put(abi.AddrKey(key), &sum)
}

// Subtracts up to the specified amount from a balance, without reducing the balance below some minimum.
// Returns the amount subtracted.
func (t *BalanceTable) SubtractWithMinimum(key addr.Address, req abi.TokenAmount, floor abi.TokenAmount) (abi.TokenAmount, error) {
	prev, err := t.Get(key)
	if err != nil {
		return big.Zero(), err
	}

	available := big.Max(big.Zero(), big.Sub(prev, floor))
	sub := big.Min(available, req)
	if sub.Sign() > 0 {
		err = t.Add(key, sub.Neg())
		if err != nil {
			return big.Zero(), err
		}
	}
	return sub, nil
}

// MustSubtract subtracts the given amount from the account's balance.
// Returns an error if the account has insufficient balance
func (t *BalanceTable) MustSubtract(key addr.Address, req abi.TokenAmount) error {
	prev, err := t.Get(key)
	if err != nil {
		return err
	}
	if req.GreaterThan(prev) {
		return xerrors.New("couldn't subtract the requested amount")
	}
	return t.Add(key, req.Neg())
}

// Returns the total balance held by this BalanceTable
func (t *BalanceTable) Total() (abi.TokenAmount, error) {
	total := big.Zero()
	var cur abi.TokenAmount
	err := (*Map)(t).ForEach(&cur, func(key string) error {
		total = big.Add(total, cur)
		return nil
	})
	return total, err
}
