package market

import (
	market0 "github.com/filecoin-project/specs-actors/actors/builtin/market"
)

//var PieceCIDPrefix = cid.Prefix{
//	Version:  1,
//	Codec:    cid.FilCommitmentUnsealed,
//	MhType:   mh.SHA2_256_TRUNC254_PADDED,
//	MhLength: 32,
//}
var PieceCIDPrefix = market0.PieceCIDPrefix

// Note: Deal Collateral is only released and returned to clients and miners
// when the storage deal stops counting towards power. In the current iteration,
// it will be released when the sector containing the storage deals expires,
// even though some storage deals can expire earlier than the sector does.
// Collaterals are denominated in PerEpoch to incur a cost for self dealing or
// minimal deals that last for a long time.
// Note: ClientCollateralPerEpoch may not be needed and removed pending future confirmation.
// There will be a Minimum value for both client and provider deal collateral.
//type DealProposal struct {
//	PieceCID     cid.Cid `checked:"true"` // Checked in validateDeal, CommP
//	PieceSize    abi.PaddedPieceSize
//	VerifiedDeal bool
//	Client       addr.Address
//	Provider     addr.Address
//
//	// Label is an arbitrary client chosen label to apply to the deal
//	// TODO: Limit the size of this: https://github.com/filecoin-project/specs-actors/issues/897
//	Label string
//
//	// Nominal start epoch. Deal payment is linear between StartEpoch and EndEpoch,
//	// with total amount StoragePricePerEpoch * (EndEpoch - StartEpoch).
//	// Storage deal must appear in a sealed (proven) sector no later than StartEpoch,
//	// otherwise it is invalid.
//	StartEpoch           abi.ChainEpoch
//	EndEpoch             abi.ChainEpoch
//	StoragePricePerEpoch abi.TokenAmount
//
//	ProviderCollateral abi.TokenAmount
//	ClientCollateral   abi.TokenAmount
//}
type DealProposal = market0.DealProposal

// ClientDealProposal is a DealProposal signed by a client
//type ClientDealProposal struct {
//	Proposal        DealProposal
//	ClientSignature crypto.Signature
//}
type ClientDealProposal = market0.ClientDealProposal
