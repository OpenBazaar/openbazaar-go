package miner

import (
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/filecoin-project/specs-actors/v2/actors/builtin"
)

// The period over which a miner's active sectors are expected to be proven via WindowPoSt.
// This guarantees that (1) user data is proven daily, (2) user data is stored for 24h by a rational miner
// (due to Window PoSt cost assumption).
var WPoStProvingPeriod = abi.ChainEpoch(builtin.EpochsInDay) // 24 hours PARAM_SPEC

// The period between the opening and the closing of a WindowPoSt deadline in which the miner is expected to
// provide a Window PoSt proof.
// This provides a miner enough time to compute and propagate a Window PoSt proof.
var WPoStChallengeWindow = abi.ChainEpoch(30 * 60 / builtin.EpochDurationSeconds) // 30 minutes (48 per day) PARAM_SPEC

// The number of non-overlapping PoSt deadlines in a proving period.
// This spreads a miner's Window PoSt work across a proving period.
const WPoStPeriodDeadlines = uint64(48) // PARAM_SPEC

// MaxPartitionsPerDeadline is the maximum number of partitions that will be assigned to a deadline.
// For a minimum storage of upto 1Eib, we need 300 partitions per deadline.
// 48 * 32GiB * 2349 * 300 = 1.00808144 EiB
// So, to support upto 10Eib storage, we set this to 3000.
const MaxPartitionsPerDeadline = 3000

func init() {
	// Check that the challenge windows divide the proving period evenly.
	if WPoStProvingPeriod%WPoStChallengeWindow != 0 {
		panic(fmt.Sprintf("incompatible proving period %d and challenge window %d", WPoStProvingPeriod, WPoStChallengeWindow))
	}
	// Check that WPoStPeriodDeadlines is consistent with the proving period and challenge window.
	if abi.ChainEpoch(WPoStPeriodDeadlines)*WPoStChallengeWindow != WPoStProvingPeriod {
		panic(fmt.Sprintf("incompatible proving period %d and challenge window %d", WPoStProvingPeriod, WPoStChallengeWindow))
	}
}

// The maximum number of partitions that can be loaded in a single invocation.
// This limits the number of simultaneous fault, recovery, or sector-extension declarations.
// We set this to same as MaxPartitionsPerDeadline so we can process that many partitions every deadline.
const AddressedPartitionsMax = MaxPartitionsPerDeadline

// Maximum number of unique "declarations" in batch operations.
const DeclarationsMax = AddressedPartitionsMax

// The maximum number of sector infos that can be loaded in a single invocation.
// This limits the amount of state to be read in a single message execution.
const AddressedSectorsMax = 10_000 // PARAM_SPEC

// Libp2p peer info limits.
const (
	// MaxPeerIDLength is the maximum length allowed for any on-chain peer ID.
	// Most Peer IDs are expected to be less than 50 bytes.
	MaxPeerIDLength = 128 // PARAM_SPEC

	// MaxMultiaddrData is the maximum amount of data that can be stored in multiaddrs.
	MaxMultiaddrData = 1024 // PARAM_SPEC
)

// Maximum size of a single prove-commit proof, in bytes.
// The 1024 maximum at network version 4 was an error (the expected size is 1920).
const MaxProveCommitSizeV4 = 1024
const MaxProveCommitSizeV5 = 10240

// Maximum number of control addresses a miner may register.
const MaxControlAddresses = 10

// The maximum number of partitions that may be required to be loaded in a single invocation,
// when all the sector infos for the partitions will be loaded.
func loadPartitionsSectorsMax(partitionSectorCount uint64) uint64 {
	return min64(AddressedSectorsMax/partitionSectorCount, AddressedPartitionsMax)
}

// Epochs after which chain state is final with overwhelming probability (hence the likelihood of two fork of this size is negligible)
// This is a conservative value that is chosen via simulations of all known attacks.
const ChainFinality = abi.ChainEpoch(900) // PARAM_SPEC

// Prefix for sealed sector CIDs (CommR).
var SealedCIDPrefix = cid.Prefix{
	Version:  1,
	Codec:    cid.FilCommitmentSealed,
	MhType:   mh.POSEIDON_BLS12_381_A1_FC1,
	MhLength: 32,
}

// List of proof types which may be used when creating a new miner actor.
// This is mutable to allow configuration of testing and development networks.
var SupportedProofTypes = map[abi.RegisteredSealProof]struct{}{
	abi.RegisteredSealProof_StackedDrg32GiBV1: {},
	abi.RegisteredSealProof_StackedDrg64GiBV1: {},
}

// Maximum delay to allow between sector pre-commit and subsequent proof.
// The allowable delay depends on seal proof algorithm.
var MaxProveCommitDuration = map[abi.RegisteredSealProof]abi.ChainEpoch{
	abi.RegisteredSealProof_StackedDrg32GiBV1:  builtin.EpochsInDay + PreCommitChallengeDelay, // PARAM_SPEC
	abi.RegisteredSealProof_StackedDrg2KiBV1:   builtin.EpochsInDay + PreCommitChallengeDelay,
	abi.RegisteredSealProof_StackedDrg8MiBV1:   builtin.EpochsInDay + PreCommitChallengeDelay,
	abi.RegisteredSealProof_StackedDrg512MiBV1: builtin.EpochsInDay + PreCommitChallengeDelay,
	abi.RegisteredSealProof_StackedDrg64GiBV1:  builtin.EpochsInDay + PreCommitChallengeDelay,
}

// Maximum delay between challenge and pre-commitment.
// This prevents a miner sealing sectors far in advance of committing them to the chain, thus committing to a
// particular chain.
var MaxPreCommitRandomnessLookback = builtin.EpochsInDay + ChainFinality // PARAM_SPEC

// Number of epochs between publishing a sector pre-commitment and when the challenge for interactive PoRep is drawn.
// This (1) prevents a miner predicting a challenge before staking their pre-commit deposit, and
// (2) prevents a miner attempting a long fork in the past to insert a pre-commitment after seeing the challenge.
var PreCommitChallengeDelay = abi.ChainEpoch(150) // PARAM_SPEC

// Lookback from the deadline's challenge window opening from which to sample chain randomness for the WindowPoSt challenge seed.
// This means that deadline windows can be non-overlapping (which make the programming simpler) without requiring a
// miner to wait for chain stability during the challenge window.
// This value cannot be too large lest it compromise the rationality of honest storage (from Window PoSt cost assumptions).
const WPoStChallengeLookback = abi.ChainEpoch(20) // PARAM_SPEC

// Minimum period between fault declaration and the next deadline opening.
// If the number of epochs between fault declaration and deadline's challenge window opening is lower than FaultDeclarationCutoff,
// the fault declaration is considered invalid for that deadline.
// This guarantees that a miner is not likely to successfully fork the chain and declare a fault after seeing the challenges.
const FaultDeclarationCutoff = WPoStChallengeLookback + 50 // PARAM_SPEC

// The maximum age of a fault before the sector is terminated.
// This bounds the time a miner can lose client's data before sacrificing pledge and deal collateral.
var FaultMaxAge = WPoStProvingPeriod * 14 // PARAM_SPEC

// Staging period for a miner worker key change.
// This delay prevents a miner choosing a more favorable worker key that wins leader elections.
const WorkerKeyChangeDelay = ChainFinality // PARAM_SPEC

// Minimum number of epochs past the current epoch a sector may be set to expire.
const MinSectorExpiration = 180 * builtin.EpochsInDay // PARAM_SPEC

// The maximum number of epochs past the current epoch that sector lifetime may be extended.
// A sector may be extended multiple times, however, the total maximum lifetime is also bounded by
// the associated seal proof's maximum lifetime.
const MaxSectorExpirationExtension = 540 * builtin.EpochsInDay // PARAM_SPEC

// Ratio of sector size to maximum number of deals per sector.
// The maximum number of deals is the sector size divided by this number (2^27)
// which limits 32GiB sectors to 256 deals and 64GiB sectors to 512
const DealLimitDenominator = 134217728 // PARAM_SPEC

// Number of epochs after a consensus fault for which a miner is ineligible
// for permissioned actor methods and winning block elections.
const ConsensusFaultIneligibilityDuration = ChainFinality

// DealWeight and VerifiedDealWeight are spacetime occupied by regular deals and verified deals in a sector.
// Sum of DealWeight and VerifiedDealWeight should be less than or equal to total SpaceTime of a sector.
// Sectors full of VerifiedDeals will have a SectorQuality of VerifiedDealWeightMultiplier/QualityBaseMultiplier.
// Sectors full of Deals will have a SectorQuality of DealWeightMultiplier/QualityBaseMultiplier.
// Sectors with neither will have a SectorQuality of QualityBaseMultiplier/QualityBaseMultiplier.
// SectorQuality of a sector is a weighted average of multipliers based on their proportions.
func QualityForWeight(size abi.SectorSize, duration abi.ChainEpoch, dealWeight, verifiedWeight abi.DealWeight) abi.SectorQuality {
	// sectorSpaceTime = size * duration
	sectorSpaceTime := big.Mul(big.NewIntUnsigned(uint64(size)), big.NewInt(int64(duration)))
	// totalDealSpaceTime = dealWeight + verifiedWeight
	totalDealSpaceTime := big.Add(dealWeight, verifiedWeight)

	// Base - all size * duration of non-deals
	// weightedBaseSpaceTime = (sectorSpaceTime - totalDealSpaceTime) * QualityBaseMultiplier
	weightedBaseSpaceTime := big.Mul(big.Sub(sectorSpaceTime, totalDealSpaceTime), builtin.QualityBaseMultiplier)
	// Deal - all deal size * deal duration * 10
	// weightedDealSpaceTime = dealWeight * DealWeightMultiplier
	weightedDealSpaceTime := big.Mul(dealWeight, builtin.DealWeightMultiplier)
	// Verified - all verified deal size * verified deal duration * 100
	// weightedVerifiedSpaceTime = verifiedWeight * VerifiedDealWeightMultiplier
	weightedVerifiedSpaceTime := big.Mul(verifiedWeight, builtin.VerifiedDealWeightMultiplier)
	// Sum - sum of all spacetime
	// weightedSumSpaceTime = weightedBaseSpaceTime + weightedDealSpaceTime + weightedVerifiedSpaceTime
	weightedSumSpaceTime := big.Sum(weightedBaseSpaceTime, weightedDealSpaceTime, weightedVerifiedSpaceTime)
	// scaledUpWeightedSumSpaceTime = weightedSumSpaceTime * 2^20
	scaledUpWeightedSumSpaceTime := big.Lsh(weightedSumSpaceTime, builtin.SectorQualityPrecision)

	// Average of weighted space time: (scaledUpWeightedSumSpaceTime / sectorSpaceTime * 10)
	return big.Div(big.Div(scaledUpWeightedSumSpaceTime, sectorSpaceTime), builtin.QualityBaseMultiplier)
}

// The power for a sector size, committed duration, and weight.
func QAPowerForWeight(size abi.SectorSize, duration abi.ChainEpoch, dealWeight, verifiedWeight abi.DealWeight) abi.StoragePower {
	quality := QualityForWeight(size, duration, dealWeight, verifiedWeight)
	return big.Rsh(big.Mul(big.NewIntUnsigned(uint64(size)), quality), builtin.SectorQualityPrecision)
}

// The quality-adjusted power for a sector.
func QAPowerForSector(size abi.SectorSize, sector *SectorOnChainInfo) abi.StoragePower {
	duration := sector.Expiration - sector.Activation
	return QAPowerForWeight(size, duration, sector.DealWeight, sector.VerifiedDealWeight)
}

// Determine maximum number of deal miner's sector can hold
func SectorDealsMax(size abi.SectorSize) uint64 {
	return max64(256, uint64(size/DealLimitDenominator))
}

// Initial share of consensus fault penalty allocated as reward to the reporter.
var consensusFaultReporterInitialShare = builtin.BigFrac{
	Numerator:   big.NewInt(1), // PARAM_SPEC
	Denominator: big.NewInt(1000),
}

// Per-epoch growth rate of the consensus fault reporter reward.
// consensusFaultReporterInitialShare*consensusFaultReporterShareGrowthRate^consensusFaultGrowthDuration ~= consensusFaultReporterMaxShare
// consensusFaultGrowthDuration = 500 epochs
var consensusFaultReporterShareGrowthRate = builtin.BigFrac{
	Numerator:   big.NewInt(100785473384), // PARAM_SPEC
	Denominator: big.NewInt(100000000000),
}

// Maximum share of consensus fault penalty allocated as reporter reward.
var consensusFaultMaxReporterShare = builtin.BigFrac{
	Numerator:   big.NewInt(1), // PARAM_SPEC
	Denominator: big.NewInt(20),
}

// Specification for a linear vesting schedule.
type VestSpec struct {
	InitialDelay abi.ChainEpoch // Delay before any amount starts vesting.
	VestPeriod   abi.ChainEpoch // Period over which the total should vest, after the initial delay.
	StepDuration abi.ChainEpoch // Duration between successive incremental vests (independent of vesting period).
	Quantization abi.ChainEpoch // Maximum precision of vesting table (limits cardinality of table).
}

// The vesting schedule for total rewards (block reward + gas reward) earned by a block producer.
var RewardVestingSpec = VestSpec{ // PARAM_SPEC
	InitialDelay: abi.ChainEpoch(0),
	VestPeriod:   abi.ChainEpoch(180 * builtin.EpochsInDay),
	StepDuration: abi.ChainEpoch(1 * builtin.EpochsInDay),
	Quantization: 12 * builtin.EpochsInHour,
}

// When an actor reports a consensus fault, they earn a share of the penalty paid by the miner.
// This amount is:  Min(initialShare * growthRate^elapsed, maxReporterShare) * collateral
// The reward grows over time until a maximum, forming an auction for the report.
func RewardForConsensusSlashReport(elapsedEpoch abi.ChainEpoch, collateral abi.TokenAmount) abi.TokenAmount {
	// High level description
	// var growthRate = SLASHER_SHARE_GROWTH_RATE_NUM / SLASHER_SHARE_GROWTH_RATE_DENOM
	// var multiplier = growthRate^elapsedEpoch
	// var slasherProportion = min(INITIAL_SLASHER_SHARE * multiplier, 0.05)
	// return collateral * slasherProportion

	// BigInt Operation
	// NUM = SLASHER_SHARE_GROWTH_RATE_NUM^elapsedEpoch * INITIAL_SLASHER_SHARE_NUM * collateral
	// DENOM = SLASHER_SHARE_GROWTH_RATE_DENOM^elapsedEpoch * INITIAL_SLASHER_SHARE_DENOM
	// slasher_amount = min(NUM/DENOM, collateral)
	elapsed := big.NewInt(int64(elapsedEpoch))

	// The following is equivalent to: slasherShare = growthRate^elapsed
	// slasherShareNumerator = growthRateNumerator^elapsed
	slasherShareNumerator := big.Exp(consensusFaultReporterShareGrowthRate.Numerator, elapsed)
	// slasherShareDenominator = growthRateDenominator^elapsed
	slasherShareDenominator := big.Exp(consensusFaultReporterShareGrowthRate.Denominator, elapsed)

	// The following is equivalent to: reward = slasherShare * initialShare * collateral
	// num = slasherShareNumerator * initialShareNumerator * collateral
	num := big.Mul(big.Mul(slasherShareNumerator, consensusFaultReporterInitialShare.Numerator), collateral)
	// denom = slasherShareDenominator * initialShareDenominator
	denom := big.Mul(slasherShareDenominator, consensusFaultReporterInitialShare.Denominator)

	// The following is equivalent to: Min(reward, collateral * maxReporterShare)
	// Min(rewardNum/rewardDenom, maxReporterShareNum/maxReporterShareDen*collateral)
	return big.Min(big.Div(num, denom), big.Div(big.Mul(collateral, consensusFaultMaxReporterShare.Numerator),
		consensusFaultMaxReporterShare.Denominator))
}
