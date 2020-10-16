package builtin

import (
	"github.com/filecoin-project/go-state-types/abi"
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
)

const (
	MethodSend        = builtin0.MethodSend
	MethodConstructor = builtin0.MethodConstructor
)

var MethodsAccount = struct {
	Constructor   abi.MethodNum
	PubkeyAddress abi.MethodNum
}{MethodConstructor, 2}

var MethodsInit = struct {
	Constructor abi.MethodNum
	Exec        abi.MethodNum
}{MethodConstructor, 2}

var MethodsCron = struct {
	Constructor abi.MethodNum
	EpochTick   abi.MethodNum
}{MethodConstructor, 2}

var MethodsReward = struct {
	Constructor      abi.MethodNum
	AwardBlockReward abi.MethodNum
	ThisEpochReward  abi.MethodNum
	UpdateNetworkKPI abi.MethodNum
}{MethodConstructor, 2, 3, 4}

var MethodsMultisig = struct {
	Constructor                 abi.MethodNum
	Propose                     abi.MethodNum
	Approve                     abi.MethodNum
	Cancel                      abi.MethodNum
	AddSigner                   abi.MethodNum
	RemoveSigner                abi.MethodNum
	SwapSigner                  abi.MethodNum
	ChangeNumApprovalsThreshold abi.MethodNum
	LockBalance                 abi.MethodNum
}{MethodConstructor, 2, 3, 4, 5, 6, 7, 8, 9}

var MethodsPaych = struct {
	Constructor        abi.MethodNum
	UpdateChannelState abi.MethodNum
	Settle             abi.MethodNum
	Collect            abi.MethodNum
}{MethodConstructor, 2, 3, 4}

var MethodsMarket = struct {
	Constructor              abi.MethodNum
	AddBalance               abi.MethodNum
	WithdrawBalance          abi.MethodNum
	PublishStorageDeals      abi.MethodNum
	VerifyDealsForActivation abi.MethodNum
	ActivateDeals            abi.MethodNum
	OnMinerSectorsTerminate  abi.MethodNum
	ComputeDataCommitment    abi.MethodNum
	CronTick                 abi.MethodNum
}{MethodConstructor, 2, 3, 4, 5, 6, 7, 8, 9}

var MethodsPower = struct {
	Constructor              abi.MethodNum
	CreateMiner              abi.MethodNum
	UpdateClaimedPower       abi.MethodNum
	EnrollCronEvent          abi.MethodNum
	OnEpochTickEnd           abi.MethodNum
	UpdatePledgeTotal        abi.MethodNum
	Deprecated1              abi.MethodNum
	SubmitPoRepForBulkVerify abi.MethodNum
	CurrentTotalPower        abi.MethodNum
}{MethodConstructor, 2, 3, 4, 5, 6, 7, 8, 9}

var MethodsMiner = struct {
	Constructor              abi.MethodNum
	ControlAddresses         abi.MethodNum
	ChangeWorkerAddress      abi.MethodNum
	ChangePeerID             abi.MethodNum
	SubmitWindowedPoSt       abi.MethodNum
	PreCommitSector          abi.MethodNum
	ProveCommitSector        abi.MethodNum
	ExtendSectorExpiration   abi.MethodNum
	TerminateSectors         abi.MethodNum
	DeclareFaults            abi.MethodNum
	DeclareFaultsRecovered   abi.MethodNum
	OnDeferredCronEvent      abi.MethodNum
	CheckSectorProven        abi.MethodNum
	ApplyRewards             abi.MethodNum
	ReportConsensusFault     abi.MethodNum
	WithdrawBalance          abi.MethodNum
	ConfirmSectorProofsValid abi.MethodNum
	ChangeMultiaddrs         abi.MethodNum
	CompactPartitions        abi.MethodNum
	CompactSectorNumbers     abi.MethodNum
	ConfirmUpdateWorkerKey   abi.MethodNum
	RepayDebt                abi.MethodNum
	ChangeOwnerAddress       abi.MethodNum
}{MethodConstructor, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23}

var MethodsVerifiedRegistry = struct {
	Constructor       abi.MethodNum
	AddVerifier       abi.MethodNum
	RemoveVerifier    abi.MethodNum
	AddVerifiedClient abi.MethodNum
	UseBytes          abi.MethodNum
	RestoreBytes      abi.MethodNum
}{MethodConstructor, 2, 3, 4, 5, 6}
