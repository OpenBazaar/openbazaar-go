package builtin

import (
	addr "github.com/filecoin-project/go-address"
	autil "github.com/filecoin-project/specs-actors/actors/util"
	"github.com/ipfs/go-cid"
)

// Addresses for singleton system actors.
var (
	// Distinguished AccountActor that is the source of system implicit messages.
	SystemActorAddr           = mustMakeAddress(0)
	InitActorAddr             = mustMakeAddress(1)
	RewardActorAddr           = mustMakeAddress(2)
	CronActorAddr             = mustMakeAddress(3)
	StoragePowerActorAddr     = mustMakeAddress(4)
	StorageMarketActorAddr    = mustMakeAddress(5)
	VerifiedRegistryActorAddr = mustMakeAddress(6)
	// Distinguished AccountActor that is the destination of all burnt funds.
	BurntFundsActorAddr = mustMakeAddress(99)
)

const FirstNonSingletonActorId = 100

func mustMakeAddress(id uint64) addr.Address {
	address, err := addr.NewIDAddress(id)
	autil.AssertNoError(err)
	return address
}

// IsSingletonActor returns true if the code belongs to a singleton actor.
func IsSingletonActor(code cid.Cid) bool {
	return code.Equals(SystemActorCodeID) ||
		code.Equals(InitActorCodeID) ||
		code.Equals(RewardActorCodeID) ||
		code.Equals(CronActorCodeID) ||
		code.Equals(StoragePowerActorCodeID) ||
		code.Equals(StorageMarketActorCodeID) ||
		code.Equals(VerifiedRegistryActorCodeID)
}
