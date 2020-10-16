package miner

import (
	"fmt"

	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	xc "github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/specs-actors/actors/util/adt"
)

func LoadSectors(store adt.Store, root cid.Cid) (Sectors, error) {
	sectorsArr, err := adt.AsArray(store, root)
	if err != nil {
		return Sectors{}, err
	}
	return Sectors{sectorsArr}, nil
}

// Sectors is a helper type for accessing/modifying a miner's sectors. It's safe
// to pass this object around as needed.
type Sectors struct {
	*adt.Array
}

func (sa Sectors) Load(sectorNos bitfield.BitField) ([]*SectorOnChainInfo, error) {
	var sectorInfos []*SectorOnChainInfo
	if err := sectorNos.ForEach(func(i uint64) error {
		var sectorOnChain SectorOnChainInfo
		found, err := sa.Array.Get(i, &sectorOnChain)
		if err != nil {
			return xc.ErrIllegalState.Wrapf("failed to load sector %v: %w", abi.SectorNumber(i), err)
		} else if !found {
			return xc.ErrNotFound.Wrapf("can't find sector %d", i)
		}
		sectorInfos = append(sectorInfos, &sectorOnChain)
		return nil
	}); err != nil {
		// Keep the underlying error code, unless the error was from
		// traversing the bitfield. In that case, it's an illegal
		// argument error.
		return nil, xc.Unwrap(err, xc.ErrIllegalArgument).Wrapf("failed to load sectors: %w", err)
	}
	return sectorInfos, nil
}

func (sa Sectors) Get(sectorNumber abi.SectorNumber) (info *SectorOnChainInfo, found bool, err error) {
	var res SectorOnChainInfo
	if found, err := sa.Array.Get(uint64(sectorNumber), &res); err != nil {
		return nil, false, xerrors.Errorf("failed to get sector %d: %w", sectorNumber, err)
	} else if !found {
		return nil, false, nil
	}
	return &res, true, nil
}

func (sa Sectors) Store(infos ...*SectorOnChainInfo) error {
	for _, info := range infos {
		if info == nil {
			return xerrors.Errorf("nil sector info")
		}
		if info.SectorNumber > abi.MaxSectorNumber {
			return fmt.Errorf("sector number %d out of range", info.SectorNumber)
		}
		if err := sa.Set(uint64(info.SectorNumber), info); err != nil {
			return fmt.Errorf("failed to store sector %d: %w", info.SectorNumber, err)
		}
	}

	if sa.Length() > SectorsMax {
		return xerrors.Errorf("too many sectors")
	}

	return nil
}

func (sa Sectors) MustGet(sectorNumber abi.SectorNumber) (info *SectorOnChainInfo, err error) {
	if info, found, err := sa.Get(sectorNumber); err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("sector %d not found", sectorNumber)
	} else {
		return info, nil
	}
}
