// Package clientutils provides utility functions for the storage client & client FSM
package clientutils

import (
	"context"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-fil-markets/pieceio"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
)

// CommP calculates the commP for a given dataref
func CommP(ctx context.Context, pieceIO pieceio.PieceIO, rt abi.RegisteredSealProof, data *storagemarket.DataRef) (cid.Cid, abi.UnpaddedPieceSize, error) {
	if data.PieceCid != nil {
		return *data.PieceCid, data.PieceSize, nil
	}

	if data.TransferType == storagemarket.TTManual {
		return cid.Undef, 0, xerrors.New("Piece CID and size must be set for manual transfer")
	}

	commp, paddedSize, err := pieceIO.GeneratePieceCommitment(rt, data.Root, shared.AllSelector())
	if err != nil {
		return cid.Undef, 0, xerrors.Errorf("generating CommP: %w", err)
	}

	return commp, paddedSize, nil
}

// VerifyFunc is a function that can validate a signature for a given address and bytes
type VerifyFunc func(context.Context, crypto.Signature, address.Address, []byte, shared.TipSetToken) (bool, error)

// VerifyResponse verifies the signature on the given signed response matches
// the given miner address, using the given signature verification function
func VerifyResponse(ctx context.Context, resp network.SignedResponse, minerAddr address.Address, tok shared.TipSetToken, verifier VerifyFunc) error {
	b, err := cborutil.Dump(&resp.Response)
	if err != nil {
		return err
	}
	verified, err := verifier(ctx, *resp.Signature, minerAddr, b, tok)
	if err != nil {
		return err
	}

	if !verified {
		return xerrors.New("could not verify signature")
	}

	return nil
}
