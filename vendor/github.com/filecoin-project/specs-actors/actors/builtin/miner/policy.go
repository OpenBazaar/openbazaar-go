package miner

import (
	"fmt"

	abi "github.com/filecoin-project/go-state-types/abi"
	big "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	. "github.com/filecoin-project/specs-actors/actors/util"
)

// The period over which all a miner's active sectors will be challenged.
var WPoStProvingPeriod = abi.ChainEpoch(builtin.EpochsInDay) // 24 hours

// The duration of a deadline's challenge window, the period before a deadline when the challenge is available.
var WPoStChallengeWindow = abi.ChainEpoch(30 * 60 / builtin.EpochDurationSeconds) // 30 minutes (48 per day)

// The number of non-overlapping PoSt deadlines in each proving period.
const WPoStPeriodDeadlines = uint64(48)

// WPoStMaxChainCommitAge is the maximum distance back that a valid Window PoSt must commit to the current chain.
var WPoStMaxChainCommitAge = WPoStChallengeWindow

func init() {
	// Check that the challenge windows divide the proving period evenly.
	if WPoStProvingPeriod%WPoStChallengeWindow != 0 {
		panic(fmt.Sprintf("incompatible proving period %d and challenge window %d", WPoStProvingPeriod, WPoStChallengeWindow))
	}
	if abi.ChainEpoch(WPoStPeriodDeadlines)*WPoStChallengeWindow != WPoStProvingPeriod {
		panic(fmt.Sprintf("incompatible proving period %d and challenge window %d", WPoStProvingPeriod, WPoStChallengeWindow))
	}
}

// The maximum number of sectors that a miner can have simultaneously active.
// This also bounds the number of faults that can be declared, etc.
// TODO raise this number, carefully
// https://github.com/filecoin-project/specs-actors/issues/470
const SectorsMax = 32 << 20 // PARAM_FINISH

// The maximum number of partitions that may be required to be loaded in a single invocation.
// This limits the number of simultaneous fault, recovery, or sector-extension declarations.
// With 48 deadlines (half-hour), 200 partitions per declaration permits loading a full EiB of 32GiB
// sectors with 1 message per epoch within a single half-hour deadline. A miner can of course submit more messages.
const AddressedPartitionsMax = 200

// The maximum number of sector infos that may be required to be loaded in a single invocation.
const AddressedSectorsMax = 10_000

// The maximum number of partitions that may be required to be loaded in a single invocation,
// when all the sector infos for the partitions will be loaded.
func loadPartitionsSectorsMax(partitionSectorCount uint64) uint64 {
	return min64(AddressedSectorsMax/partitionSectorCount, AddressedPartitionsMax)
}

// The maximum number of new sectors that may be staged by a miner during a single proving period.
const NewSectorsPerPeriodMax = 128 << 10

// Epochs after which chain state is final.
const ChainFinality = abi.ChainEpoch(900)

var SealedCIDPrefix = cid.Prefix{
	Version:  1,
	Codec:    cid.FilCommitmentSealed,
	MhType:   mh.POSEIDON_BLS12_381_A1_FC1,
	MhLength: 32,
}

// List of proof types which can be used when creating new miner actors
var SupportedProofTypes = map[abi.RegisteredSealProof]struct{}{
	abi.RegisteredSealProof_StackedDrg32GiBV1: {},
	abi.RegisteredSealProof_StackedDrg64GiBV1: {},
}

// Maximum duration to allow for the sealing process for seal algorithms.
// Dependent on algorithm and sector size
var MaxSealDuration = map[abi.RegisteredSealProof]abi.ChainEpoch{
	abi.RegisteredSealProof_StackedDrg32GiBV1:  abi.ChainEpoch(10000), // PARAM_FINISH
	abi.RegisteredSealProof_StackedDrg2KiBV1:   abi.ChainEpoch(10000),
	abi.RegisteredSealProof_StackedDrg8MiBV1:   abi.ChainEpoch(10000),
	abi.RegisteredSealProof_StackedDrg512MiBV1: abi.ChainEpoch(10000),
	abi.RegisteredSealProof_StackedDrg64GiBV1:  abi.ChainEpoch(10000),
}

// Number of epochs between publishing the precommit and when the challenge for interactive PoRep is drawn
// used to ensure it is not predictable by miner.
var PreCommitChallengeDelay = abi.ChainEpoch(150)

// Lookback from the current epoch for state view for leader elections.
const ElectionLookback = abi.ChainEpoch(1) // PARAM_FINISH

// Lookback from the deadline's challenge window opening from which to sample chain randomness for the challenge seed.
// This lookback exists so that deadline windows can be non-overlapping (which make the programming simpler)
// but without making the miner wait for chain stability before being able to start on PoSt computation.
// The challenge is available this many epochs before the window is actually open to receiving a PoSt.
const WPoStChallengeLookback = abi.ChainEpoch(20)

// Minimum period before a deadline's challenge window opens that a fault must be declared for that deadline.
// This lookback must not be less than WPoStChallengeLookback lest a malicious miner be able to selectively declare
// faults after learning the challenge value.
const FaultDeclarationCutoff = WPoStChallengeLookback + 50

// The maximum age of a fault before the sector is terminated.
var FaultMaxAge = WPoStProvingPeriod * 14

// Staging period for a miner worker key change.
// Finality is a harsh delay for a miner who has lost their worker key, as the miner will miss Window PoSts until
// it can be changed. It's the only safe value, though. We may implement a mitigation mechanism such as a second
// key or allowing the owner account to submit PoSts while a key change is pending.
const WorkerKeyChangeDelay = ChainFinality

// Minimum number of epochs past the current epoch a sector may be set to expire.
const MinSectorExpiration = 180 * builtin.EpochsInDay

// Maximum number of epochs past the current epoch a sector may be set to expire.
// The actual maximum extension will be the minimum of CurrEpoch + MaximumSectorExpirationExtension
// and sector.ActivationEpoch+sealProof.SectorMaximumLifetime()
const MaxSectorExpirationExtension = 540 * builtin.EpochsInDay

// Ratio of sector size to maximum deals per sector.
// The maximum number of deals is the sector size divided by this number (2^27)
// which limits 32GiB sectors to 256 deals and 64GiB sectors to 512
const DealLimitDenominator = 134217728

// DealWeight and VerifiedDealWeight are spacetime occupied by regular deals and verified deals in a sector.
// Sum of DealWeight and VerifiedDealWeight should be less than or equal to total SpaceTime of a sector.
// Sectors full of VerifiedDeals will have a SectorQuality of VerifiedDealWeightMultiplier/QualityBaseMultiplier.
// Sectors full of Deals will have a SectorQuality of DealWeightMultiplier/QualityBaseMultiplier.
// Sectors with neither will have a SectorQuality of QualityBaseMultiplier/QualityBaseMultiplier.
// SectorQuality of a sector is a weighted average of multipliers based on their propotions.
func QualityForWeight(size abi.SectorSize, duration abi.ChainEpoch, dealWeight, verifiedWeight abi.DealWeight) abi.SectorQuality {
	sectorSpaceTime := big.Mul(big.NewIntUnsigned(uint64(size)), big.NewInt(int64(duration)))
	totalDealSpaceTime := big.Add(dealWeight, verifiedWeight)
	Assert(sectorSpaceTime.GreaterThanEqual(totalDealSpaceTime))

	weightedBaseSpaceTime := big.Mul(big.Sub(sectorSpaceTime, totalDealSpaceTime), builtin.QualityBaseMultiplier)
	weightedDealSpaceTime := big.Mul(dealWeight, builtin.DealWeightMultiplier)
	weightedVerifiedSpaceTime := big.Mul(verifiedWeight, builtin.VerifiedDealWeightMultiplier)
	weightedSumSpaceTime := big.Sum(weightedBaseSpaceTime, weightedDealSpaceTime, weightedVerifiedSpaceTime)
	scaledUpWeightedSumSpaceTime := big.Lsh(weightedSumSpaceTime, builtin.SectorQualityPrecision)

	return big.Div(big.Div(scaledUpWeightedSumSpaceTime, sectorSpaceTime), builtin.QualityBaseMultiplier)
}

// Returns the power for a sector size and weight.
func QAPowerForWeight(size abi.SectorSize, duration abi.ChainEpoch, dealWeight, verifiedWeight abi.DealWeight) abi.StoragePower {
	quality := QualityForWeight(size, duration, dealWeight, verifiedWeight)
	return big.Rsh(big.Mul(big.NewIntUnsigned(uint64(size)), quality), builtin.SectorQualityPrecision)
}

// Returns the quality-adjusted power for a sector.
func QAPowerForSector(size abi.SectorSize, sector *SectorOnChainInfo) abi.StoragePower {
	duration := sector.Expiration - sector.Activation
	return QAPowerForWeight(size, duration, sector.DealWeight, sector.VerifiedDealWeight)
}

// Determine maximum number of deal miner's sector can hold
func dealPerSectorLimit(size abi.SectorSize) uint64 {
	return max64(256, uint64(size/DealLimitDenominator))
}

type BigFrac struct {
	numerator   big.Int
	denominator big.Int
}

var consensusFaultReporterInitialShare = BigFrac{
	// PARAM_FINISH
	numerator:   big.NewInt(1),
	denominator: big.NewInt(1000),
}
var consensusFaultReporterShareGrowthRate = BigFrac{
	// PARAM_FINISH
	numerator:   big.NewInt(101251),
	denominator: big.NewInt(100000),
}

// Specification for a linear vesting schedule.
type VestSpec struct {
	InitialDelay abi.ChainEpoch // Delay before any amount starts vesting.
	VestPeriod   abi.ChainEpoch // Period over which the total should vest, after the initial delay.
	StepDuration abi.ChainEpoch // Duration between successive incremental vests (independent of vesting period).
	Quantization abi.ChainEpoch // Maximum precision of vesting table (limits cardinality of table).
}

var RewardVestingSpecV0 = VestSpec{
	InitialDelay: abi.ChainEpoch(20 * builtin.EpochsInDay),  // PARAM_FINISH
	VestPeriod:   abi.ChainEpoch(180 * builtin.EpochsInDay), // PARAM_FINISH
	StepDuration: abi.ChainEpoch(1 * builtin.EpochsInDay),   // PARAM_FINISH
	Quantization: 12 * builtin.EpochsInHour,                 // PARAM_FINISH
}

var RewardVestingSpecV1 = VestSpec{
	InitialDelay: abi.ChainEpoch(0),                         // PARAM_FINISH
	VestPeriod:   abi.ChainEpoch(180 * builtin.EpochsInDay), // PARAM_FINISH
	StepDuration: abi.ChainEpoch(1 * builtin.EpochsInDay),   // PARAM_FINISH
	Quantization: 12 * builtin.EpochsInHour,                 // PARAM_FINISH
}

func RewardForConsensusSlashReport(elapsedEpoch abi.ChainEpoch, collateral abi.TokenAmount) abi.TokenAmount {
	// PARAM_FINISH
	// var growthRate = SLASHER_SHARE_GROWTH_RATE_NUM / SLASHER_SHARE_GROWTH_RATE_DENOM
	// var multiplier = growthRate^elapsedEpoch
	// var slasherProportion = min(INITIAL_SLASHER_SHARE * multiplier, 1.0)
	// return collateral * slasherProportion

	// BigInt Operation
	// NUM = SLASHER_SHARE_GROWTH_RATE_NUM^elapsedEpoch * INITIAL_SLASHER_SHARE_NUM * collateral
	// DENOM = SLASHER_SHARE_GROWTH_RATE_DENOM^elapsedEpoch * INITIAL_SLASHER_SHARE_DENOM
	// slasher_amount = min(NUM/DENOM, collateral)
	maxReporterShareNum := big.NewInt(1)
	maxReporterShareDen := big.NewInt(2)

	elapsed := big.NewInt(int64(elapsedEpoch))
	slasherShareNumerator := big.Exp(consensusFaultReporterShareGrowthRate.numerator, elapsed)
	slasherShareDenominator := big.Exp(consensusFaultReporterShareGrowthRate.denominator, elapsed)

	num := big.Mul(big.Mul(slasherShareNumerator, consensusFaultReporterInitialShare.numerator), collateral)
	denom := big.Mul(slasherShareDenominator, consensusFaultReporterInitialShare.denominator)
	return big.Min(big.Div(num, denom), big.Div(big.Mul(collateral, maxReporterShareNum), maxReporterShareDen))
}
