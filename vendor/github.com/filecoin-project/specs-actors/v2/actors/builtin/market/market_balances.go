package market

import (
	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"golang.org/x/xerrors"

	. "github.com/filecoin-project/specs-actors/v2/actors/util"
)

// if the returned error is not nil, the Runtime will exit with the returned exit code.
// if the error is nil, we don't care about the exitcode.
func (m *marketStateMutation) lockClientAndProviderBalances(proposal *DealProposal) (error, exitcode.ExitCode) {
	err, code := m.maybeLockBalance(proposal.Client, proposal.ClientBalanceRequirement())
	if err != nil {
		return xerrors.Errorf("failed to lock client funds: %w", err), code
	}

	err, code = m.maybeLockBalance(proposal.Provider, proposal.ProviderCollateral)
	if err != nil {
		return xerrors.Errorf("failed to lock provider funds: %w", err), code
	}

	m.totalClientLockedCollateral = big.Add(m.totalClientLockedCollateral, proposal.ClientCollateral)
	m.totalClientStorageFee = big.Add(m.totalClientStorageFee, proposal.TotalStorageFee())
	m.totalProviderLockedCollateral = big.Add(m.totalProviderLockedCollateral, proposal.ProviderCollateral)

	return nil, exitcode.Ok
}

func (m *marketStateMutation) unlockBalance(addr addr.Address, amount abi.TokenAmount, lockReason BalanceLockingReason) error {
	Assert(amount.GreaterThanEqual(big.Zero()))

	err := m.lockedTable.MustSubtract(addr, amount)
	if err != nil {
		return xerrors.Errorf("subtracting from locked balance: %v", err)
	}

	switch lockReason {
	case ClientCollateral:
		m.totalClientLockedCollateral = big.Sub(m.totalClientLockedCollateral, amount)
	case ClientStorageFee:
		m.totalClientStorageFee = big.Sub(m.totalClientStorageFee, amount)
	case ProviderCollateral:
		m.totalProviderLockedCollateral = big.Sub(m.totalProviderLockedCollateral, amount)
	}

	return nil
}

// move funds from locked in client to available in provider
func (m *marketStateMutation) transferBalance(rt Runtime, fromAddr addr.Address, toAddr addr.Address, amount abi.TokenAmount) {
	Assert(amount.GreaterThanEqual(big.Zero()))

	if err := m.escrowTable.MustSubtract(fromAddr, amount); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "subtract from escrow: %v", err)
	}

	if err := m.unlockBalance(fromAddr, amount, ClientStorageFee); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "subtract from locked: %v", err)
	}

	if err := m.escrowTable.Add(toAddr, amount); err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "add to escrow: %v", err)
	}
}

func (m *marketStateMutation) slashBalance(addr addr.Address, amount abi.TokenAmount, reason BalanceLockingReason) error {
	Assert(amount.GreaterThanEqual(big.Zero()))

	if err := m.escrowTable.MustSubtract(addr, amount); err != nil {
		return xerrors.Errorf("subtract from escrow: %v", err)
	}

	return m.unlockBalance(addr, amount, reason)
}

func (m *marketStateMutation) maybeLockBalance(addr addr.Address, amount abi.TokenAmount) (error, exitcode.ExitCode) {
	Assert(amount.GreaterThanEqual(big.Zero()))

	prevLocked, err := m.lockedTable.Get(addr)
	if err != nil {
		return xerrors.Errorf("failed to get locked balance: %w", err), exitcode.ErrIllegalState
	}

	escrowBalance, err := m.escrowTable.Get(addr)
	if err != nil {
		return xerrors.Errorf("failed to get escrow balance: %w", err), exitcode.ErrIllegalState
	}

	if big.Add(prevLocked, amount).GreaterThan(escrowBalance) {
		return xerrors.Errorf("not enough balance to lock for addr %s: escrow balance %s < locked %s + required %s", addr, escrowBalance, prevLocked, amount),
			exitcode.ErrInsufficientFunds
	}

	if err := m.lockedTable.Add(addr, amount); err != nil {
		return xerrors.Errorf("failed to add locked balance: %w", err), exitcode.ErrIllegalState
	}
	return nil, exitcode.Ok
}
