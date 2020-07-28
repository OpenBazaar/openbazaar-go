package exported

import (
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	cid "github.com/ipfs/go-cid"

	"github.com/filecoin-project/specs-actors/actors/builtin/account"
	"github.com/filecoin-project/specs-actors/actors/builtin/cron"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/builtin/multisig"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/actors/builtin/reward"
	"github.com/filecoin-project/specs-actors/actors/builtin/system"
	"github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
)

var _ abi.Invokee = BuiltinActor{}

type BuiltinActor struct {
	actor abi.Invokee
	code  cid.Cid
}

// Code is the CodeID (cid) of the actor.
func (b BuiltinActor) Code() cid.Cid {
	return b.code
}

// Exports returns a slice of callable Actor methods.
func (b BuiltinActor) Exports() []interface{} {
	return b.actor.Exports()
}

func BuiltinActors() []BuiltinActor {
	return []BuiltinActor{
		{
			actor: account.Actor{},
			code:  builtin.AccountActorCodeID,
		},
		{
			actor: cron.Actor{},
			code:  builtin.CronActorCodeID,
		},
		{
			actor: init_.Actor{},
			code:  builtin.InitActorCodeID,
		},
		{
			actor: market.Actor{},
			code:  builtin.StorageMarketActorCodeID,
		},
		{
			actor: miner.Actor{},
			code:  builtin.StorageMinerActorCodeID,
		},
		{
			actor: multisig.Actor{},
			code:  builtin.MultisigActorCodeID,
		},
		{
			actor: paych.Actor{},
			code:  builtin.PaymentChannelActorCodeID,
		},
		{
			actor: power.Actor{},
			code:  builtin.StoragePowerActorCodeID,
		},
		{
			actor: reward.Actor{},
			code:  builtin.RewardActorCodeID,
		},
		{
			actor: system.Actor{},
			code:  builtin.SystemActorCodeID,
		},
		{
			actor: verifreg.Actor{},
			code:  builtin.VerifiedRegistryActorCodeID,
		},
	}
}
