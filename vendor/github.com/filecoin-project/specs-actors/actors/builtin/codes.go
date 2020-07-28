package builtin

import (
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// The built-in actor code IDs
var (
	SystemActorCodeID           cid.Cid
	InitActorCodeID             cid.Cid
	CronActorCodeID             cid.Cid
	AccountActorCodeID          cid.Cid
	StoragePowerActorCodeID     cid.Cid
	StorageMinerActorCodeID     cid.Cid
	StorageMarketActorCodeID    cid.Cid
	PaymentChannelActorCodeID   cid.Cid
	MultisigActorCodeID         cid.Cid
	RewardActorCodeID           cid.Cid
	VerifiedRegistryActorCodeID cid.Cid
	CallerTypesSignable         []cid.Cid
)

func init() {
	builder := cid.V1Builder{Codec: cid.Raw, MhType: mh.IDENTITY}
	makeBuiltin := func(s string) cid.Cid {
		c, err := builder.Sum([]byte(s))
		if err != nil {
			panic(err)
		}
		return c
	}

	SystemActorCodeID = makeBuiltin("fil/1/system")
	InitActorCodeID = makeBuiltin("fil/1/init")
	CronActorCodeID = makeBuiltin("fil/1/cron")
	AccountActorCodeID = makeBuiltin("fil/1/account")
	StoragePowerActorCodeID = makeBuiltin("fil/1/storagepower")
	StorageMinerActorCodeID = makeBuiltin("fil/1/storageminer")
	StorageMarketActorCodeID = makeBuiltin("fil/1/storagemarket")
	PaymentChannelActorCodeID = makeBuiltin("fil/1/paymentchannel")
	MultisigActorCodeID = makeBuiltin("fil/1/multisig")
	RewardActorCodeID = makeBuiltin("fil/1/reward")
	VerifiedRegistryActorCodeID = makeBuiltin("fil/1/verifiedregistry")

	// Set of actor code types that can represent external signing parties.
	CallerTypesSignable = []cid.Cid{AccountActorCodeID, MultisigActorCodeID}
}

// IsBuiltinActor returns true if the code belongs to an actor defined in this repo.
func IsBuiltinActor(code cid.Cid) bool {
	return code.Equals(SystemActorCodeID) ||
		code.Equals(InitActorCodeID) ||
		code.Equals(CronActorCodeID) ||
		code.Equals(AccountActorCodeID) ||
		code.Equals(StoragePowerActorCodeID) ||
		code.Equals(StorageMinerActorCodeID) ||
		code.Equals(StorageMarketActorCodeID) ||
		code.Equals(PaymentChannelActorCodeID) ||
		code.Equals(MultisigActorCodeID) ||
		code.Equals(RewardActorCodeID) ||
		code.Equals(VerifiedRegistryActorCodeID)
}

// ActorNameByCode returns the (string) name of the actor given a cid code.
func ActorNameByCode(code cid.Cid) string {
	if !code.Defined() {
		return "<undefined>"
	}

	names := map[cid.Cid]string{
		SystemActorCodeID:         "fil/1/system",
		InitActorCodeID:           "fil/1/init",
		CronActorCodeID:           "fil/1/cron",
		AccountActorCodeID:        "fil/1/account",
		StoragePowerActorCodeID:   "fil/1/storagepower",
		StorageMinerActorCodeID:   "fil/1/storageminer",
		StorageMarketActorCodeID:  "fil/1/storagemarket",
		PaymentChannelActorCodeID: "fil/1/paymentchannel",
		MultisigActorCodeID:       "fil/1/multisig",
		RewardActorCodeID:         "fil/1/reward",
	}
	name, ok := names[code]
	if !ok {
		return "<unknown>"
	}
	return name
}

// Tests whether a code CID represents an actor that can be an external principal: i.e. an account or multisig.
// We could do something more sophisticated here: https://github.com/filecoin-project/specs-actors/issues/178
func IsPrincipal(code cid.Cid) bool {
	for _, c := range CallerTypesSignable {
		if c.Equals(code) {
			return true
		}
	}
	return false
}
