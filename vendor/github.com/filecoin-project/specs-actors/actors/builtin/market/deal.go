package market

import (
	"bytes"

	addr "github.com/filecoin-project/go-address"
	abi "github.com/filecoin-project/go-state-types/abi"
	big "github.com/filecoin-project/go-state-types/big"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	cid "github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

var PieceCIDPrefix = cid.Prefix{
	Version:  1,
	Codec:    cid.FilCommitmentUnsealed,
	MhType:   mh.SHA2_256_TRUNC254_PADDED,
	MhLength: 32,
}

// Note: Deal Collateral is only released and returned to clients and miners
// when the storage deal stops counting towards power. In the current iteration,
// it will be released when the sector containing the storage deals expires,
// even though some storage deals can expire earlier than the sector does.
// Collaterals are denominated in PerEpoch to incur a cost for self dealing or
// minimal deals that last for a long time.
// Note: ClientCollateralPerEpoch may not be needed and removed pending future confirmation.
// There will be a Minimum value for both client and provider deal collateral.
type DealProposal struct {
	PieceCID     cid.Cid `checked:"true"` // Checked in validateDeal, CommP
	PieceSize    abi.PaddedPieceSize
	VerifiedDeal bool
	Client       addr.Address
	Provider     addr.Address

	// Label is an arbitrary client chosen label to apply to the deal
	Label string

	// Nominal start epoch. Deal payment is linear between StartEpoch and EndEpoch,
	// with total amount StoragePricePerEpoch * (EndEpoch - StartEpoch).
	// Storage deal must appear in a sealed (proven) sector no later than StartEpoch,
	// otherwise it is invalid.
	StartEpoch           abi.ChainEpoch
	EndEpoch             abi.ChainEpoch
	StoragePricePerEpoch abi.TokenAmount

	ProviderCollateral abi.TokenAmount
	ClientCollateral   abi.TokenAmount
}

// ClientDealProposal is a DealProposal signed by a client
type ClientDealProposal struct {
	Proposal        DealProposal
	ClientSignature acrypto.Signature
}

func (p *DealProposal) Duration() abi.ChainEpoch {
	return p.EndEpoch - p.StartEpoch
}

func (p *DealProposal) TotalStorageFee() abi.TokenAmount {
	return big.Mul(p.StoragePricePerEpoch, big.NewInt(int64(p.Duration())))
}

func (p *DealProposal) ClientBalanceRequirement() abi.TokenAmount {
	return big.Add(p.ClientCollateral, p.TotalStorageFee())
}

func (p *DealProposal) ProviderBalanceRequirement() abi.TokenAmount {
	return p.ProviderCollateral
}

func (p *DealProposal) Cid() (cid.Cid, error) {
	buf := new(bytes.Buffer)
	if err := p.MarshalCBOR(buf); err != nil {
		return cid.Undef, err
	}
	return abi.CidBuilder.Sum(buf.Bytes())
}

type DealState struct {
	SectorStartEpoch abi.ChainEpoch // -1 if not yet included in proven sector
	LastUpdatedEpoch abi.ChainEpoch // -1 if deal state never updated
	SlashEpoch       abi.ChainEpoch // -1 if deal never slashed
}
