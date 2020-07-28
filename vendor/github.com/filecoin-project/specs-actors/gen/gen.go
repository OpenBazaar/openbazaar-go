package main

import (
	gen "github.com/whyrusleeping/cbor-gen"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	account "github.com/filecoin-project/specs-actors/actors/builtin/account"
	cron "github.com/filecoin-project/specs-actors/actors/builtin/cron"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	market "github.com/filecoin-project/specs-actors/actors/builtin/market"
	miner "github.com/filecoin-project/specs-actors/actors/builtin/miner"
	multisig "github.com/filecoin-project/specs-actors/actors/builtin/multisig"
	paych "github.com/filecoin-project/specs-actors/actors/builtin/paych"
	power "github.com/filecoin-project/specs-actors/actors/builtin/power"
	reward "github.com/filecoin-project/specs-actors/actors/builtin/reward"
	system "github.com/filecoin-project/specs-actors/actors/builtin/system"
	verifreg "github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	puppet "github.com/filecoin-project/specs-actors/actors/puppet"
)

func main() {
	// Common types
	if err := gen.WriteTupleEncodersToFile("./actors/abi/cbor_gen.go", "abi",
		abi.PieceInfo{},
		abi.SectorID{},
		abi.SectorInfo{},
		abi.SealVerifyInfo{},
		abi.PoStProof{},
		abi.WindowPoStVerifyInfo{},
		abi.WinningPoStVerifyInfo{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/cbor_gen.go", "builtin",
		builtin.MinerAddrs{},
		builtin.ConfirmSectorProofsParams{},
	); err != nil {
		panic(err)
	}

	// Actors
	if err := gen.WriteTupleEncodersToFile("./actors/builtin/system/cbor_gen.go", "system",
		// actor state
		system.State{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/account/cbor_gen.go", "account",
		// actor state
		account.State{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/init/cbor_gen.go", "init",
		// actor state
		init_.State{},
		// method params
		init_.ConstructorParams{},
		init_.ExecParams{},
		init_.ExecReturn{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/cron/cbor_gen.go", "cron",
		// actor state
		cron.State{},
		cron.Entry{},
		// method params
		cron.ConstructorParams{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/reward/cbor_gen.go", "reward",
		// actor state
		reward.State{},
		// method params
		reward.AwardBlockRewardParams{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/multisig/cbor_gen.go", "multisig",
		// actor state
		multisig.State{},
		multisig.Transaction{},
		multisig.ProposalHashData{},
		// method params
		multisig.ConstructorParams{},
		multisig.ProposeParams{},
		multisig.AddSignerParams{},
		multisig.RemoveSignerParams{},
		multisig.TxnIDParams{},
		multisig.ChangeNumApprovalsThresholdParams{},
		multisig.SwapSignerParams{},
		// method returns
		multisig.ApproveReturn{},
		multisig.ProposeReturn{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/paych/cbor_gen.go", "paych",
		// actor state
		paych.State{},
		paych.LaneState{},
		paych.Merge{},
		// method params
		paych.ConstructorParams{},
		paych.UpdateChannelStateParams{},
		paych.SignedVoucher{},
		paych.ModVerifyParams{},
		paych.PaymentVerifyParams{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/power/cbor_gen.go", "power",
		// actors state
		power.State{},
		power.Claim{},
		power.CronEvent{},
		// method params
		power.CreateMinerParams{},
		power.EnrollCronEventParams{},
		power.UpdateClaimedPowerParams{},
		// method returns
		power.CreateMinerReturn{},
		power.CurrentTotalPowerReturn{},
		// other types
		power.MinerConstructorParams{},
		power.SectorStorageWeightDesc{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/market/cbor_gen.go", "market",
		// actor state
		market.State{},

		// method params
		market.WithdrawBalanceParams{},
		market.PublishStorageDealsParams{},
		market.ActivateDealsParams{},
		market.VerifyDealsForActivationParams{},
		market.VerifyDealsForActivationReturn{},
		market.ComputeDataCommitmentParams{},
		market.OnMinerSectorsTerminateParams{},
		// method returns
		market.PublishStorageDealsReturn{},
		// other types
		market.DealProposal{},
		market.ClientDealProposal{},
		market.DealState{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/miner/cbor_gen.go", "miner",
		// actor state
		miner.State{},
		miner.MinerInfo{},
		miner.Deadlines{},
		miner.SectorPreCommitOnChainInfo{},
		miner.SectorPreCommitInfo{},
		miner.SectorOnChainInfo{},
		miner.WorkerKeyChange{},
		// method params
		// miner.ConstructorParams{},
		miner.SubmitWindowedPoStParams{},
		miner.TerminateSectorsParams{},
		miner.ChangePeerIDParams{},
		miner.ChangeMultiaddrsParams{},
		miner.ProveCommitSectorParams{},
		miner.ChangeWorkerAddressParams{},
		miner.ExtendSectorExpirationParams{},
		miner.DeclareFaultsParams{},
		miner.DeclareFaultsRecoveredParams{},
		miner.ReportConsensusFaultParams{},
		miner.GetControlAddressesReturn{},
		miner.CheckSectorProvenParams{},
		miner.WithdrawBalanceParams{},
		// other types
		miner.CronEventPayload{},
		miner.FaultDeclaration{},
		miner.RecoveryDeclaration{},
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/builtin/verifreg/cbor_gen.go", "verifreg",
		// actor state
		verifreg.State{},
		// method params
		verifreg.AddVerifierParams{},
		verifreg.AddVerifiedClientParams{},
		verifreg.UseBytesParams{},
		verifreg.RestoreBytesParams{},
		// other types
	); err != nil {
		panic(err)
	}

	if err := gen.WriteTupleEncodersToFile("./actors/puppet/cbor_gen.go", "puppet",
		// actor state
		puppet.State{},
		// method params
		puppet.SendParams{},
		puppet.SendReturn{},
	); err != nil {
		panic(err)
	}

}
