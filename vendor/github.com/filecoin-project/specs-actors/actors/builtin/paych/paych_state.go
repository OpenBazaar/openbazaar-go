package paych

import (
	addr "github.com/filecoin-project/go-address"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	big "github.com/filecoin-project/specs-actors/actors/abi/big"
)

// A given payment channel actor is established by From
// to enable off-chain microtransactions to To to be reconciled
// and tallied on chain.
type State struct {
	// Channel owner, who has funded the actor
	From addr.Address
	// Recipient of payouts from channel
	To addr.Address

	// Amount successfully redeemed through the payment channel, paid out on `Collect()`
	ToSend abi.TokenAmount

	// Height at which the channel can be `Collected`
	SettlingAt abi.ChainEpoch
	// Height before which the channel `ToSend` cannot be collected
	MinSettleHeight abi.ChainEpoch

	// Collections of lane states for the channel, maintained in ID order.
	LaneStates []*LaneState
}

// The Lane state tracks the latest (highest) voucher nonce used to merge the lane
// as well as the amount it has already redeemed.
type LaneState struct {
	ID       uint64 // Unique to this channel
	Redeemed big.Int
	Nonce    uint64
}

// Specifies which `Lane`s to be merged with what `Nonce` on channelUpdate
type Merge struct {
	Lane  uint64
	Nonce uint64
}

func ConstructState(from addr.Address, to addr.Address) *State {
	return &State{
		From:            from,
		To:              to,
		ToSend:          big.Zero(),
		SettlingAt:      0,
		MinSettleHeight: 0,
		LaneStates:      []*LaneState{},
	}
}
