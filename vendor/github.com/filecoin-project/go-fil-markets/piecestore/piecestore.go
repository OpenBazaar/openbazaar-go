package piecestore

import (
	"github.com/filecoin-project/go-statestore"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
)

// DSPiecePrefix is the name space for storing piece infos
var DSPiecePrefix = "/pieces"

// DSCIDPrefix is the name space for storing CID infos
var DSCIDPrefix = "/cid-infos"

// NewPieceStore returns a new piecestore based on the given datastore
func NewPieceStore(ds datastore.Batching) PieceStore {
	return &pieceStore{
		pieces:   statestore.New(namespace.Wrap(ds, datastore.NewKey(DSPiecePrefix))),
		cidInfos: statestore.New(namespace.Wrap(ds, datastore.NewKey(DSCIDPrefix))),
	}
}

type pieceStore struct {
	pieces   *statestore.StateStore
	cidInfos *statestore.StateStore
}

// Store `dealInfo` in the PieceStore with key `pieceCID`.
func (ps *pieceStore) AddDealForPiece(pieceCID cid.Cid, dealInfo DealInfo) error {
	return ps.mutatePieceInfo(pieceCID, func(pi *PieceInfo) error {
		for _, di := range pi.Deals {
			if di == dealInfo {
				return nil
			}
		}
		pi.Deals = append(pi.Deals, dealInfo)
		return nil
	})
}

// Store the map of blockLocations in the PieceStore's CIDInfo store, with key `pieceCID`
func (ps *pieceStore) AddPieceBlockLocations(pieceCID cid.Cid, blockLocations map[cid.Cid]BlockLocation) error {
	for c, blockLocation := range blockLocations {
		err := ps.mutateCIDInfo(c, func(ci *CIDInfo) error {
			for _, pbl := range ci.PieceBlockLocations {
				if pbl.PieceCID.Equals(pieceCID) && pbl.BlockLocation == blockLocation {
					return nil
				}
			}
			ci.PieceBlockLocations = append(ci.PieceBlockLocations, PieceBlockLocation{blockLocation, pieceCID})
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Retrieve the PieceInfo associated with `pieceCID` from the piece info store.
func (ps *pieceStore) GetPieceInfo(pieceCID cid.Cid) (PieceInfo, error) {
	var out PieceInfo
	if err := ps.pieces.Get(pieceCID).Get(&out); err != nil {
		return PieceInfo{}, err
	}
	return out, nil
}

// Retrieve the CIDInfo associated with `pieceCID` from the CID info store.
func (ps *pieceStore) GetCIDInfo(payloadCID cid.Cid) (CIDInfo, error) {
	var out CIDInfo
	if err := ps.cidInfos.Get(payloadCID).Get(&out); err != nil {
		return CIDInfo{}, err
	}
	return out, nil
}

func (ps *pieceStore) ensurePieceInfo(pieceCID cid.Cid) error {
	has, err := ps.pieces.Has(pieceCID)

	if err != nil {
		return err
	}
	if has {
		return nil
	}

	pieceInfo := PieceInfo{PieceCID: pieceCID}
	return ps.pieces.Begin(pieceCID, &pieceInfo)
}

func (ps *pieceStore) ensureCIDInfo(c cid.Cid) error {
	has, err := ps.cidInfos.Has(c)

	if err != nil {
		return err
	}

	if has {
		return nil
	}

	cidInfo := CIDInfo{CID: c}
	return ps.cidInfos.Begin(c, &cidInfo)
}

func (ps *pieceStore) mutatePieceInfo(pieceCID cid.Cid, mutator interface{}) error {
	err := ps.ensurePieceInfo(pieceCID)
	if err != nil {
		return err
	}

	return ps.pieces.Get(pieceCID).Mutate(mutator)
}

func (ps *pieceStore) mutateCIDInfo(c cid.Cid, mutator interface{}) error {
	err := ps.ensureCIDInfo(c)
	if err != nil {
		return err
	}

	return ps.cidInfos.Get(c).Mutate(mutator)
}
