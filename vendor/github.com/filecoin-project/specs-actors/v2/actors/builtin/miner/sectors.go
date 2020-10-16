package miner

import (
	"fmt"

	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	xc "github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/specs-actors/v2/actors/util/adt"
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

// Loads info for a set of sectors to be proven.
// If any of the sectors are declared faulty and not to be recovered, info for the first non-faulty sector is substituted instead.
// If any of the sectors are declared recovered, they are returned from this method.
func (sa Sectors) LoadForProof(provenSectors, expectedFaults bitfield.BitField) ([]*SectorOnChainInfo, error) {
	nonFaults, err := bitfield.SubtractBitField(provenSectors, expectedFaults)
	if err != nil {
		return nil, xerrors.Errorf("failed to diff bitfields: %w", err)
	}

	// Return empty if no non-faults
	if empty, err := nonFaults.IsEmpty(); err != nil {
		return nil, xerrors.Errorf("failed to check if bitfield was empty: %w", err)
	} else if empty {
		return nil, nil
	}

	// Select a non-faulty sector as a substitute for faulty ones.
	goodSectorNo, err := nonFaults.First()
	if err != nil {
		return nil, xerrors.Errorf("failed to get first good sector: %w", err)
	}

	// Load sector infos
	sectorInfos, err := sa.LoadWithFaultMask(provenSectors, expectedFaults, abi.SectorNumber(goodSectorNo))
	if err != nil {
		return nil, xerrors.Errorf("failed to load sector infos: %w", err)
	}
	return sectorInfos, nil
}

// Loads sector info for a sequence of sectors, substituting info for a stand-in sector for any that are faulty.
func (sa Sectors) LoadWithFaultMask(sectors bitfield.BitField, faults bitfield.BitField, faultStandIn abi.SectorNumber) ([]*SectorOnChainInfo, error) {
	standInInfo, err := sa.MustGet(faultStandIn)
	if err != nil {
		return nil, fmt.Errorf("failed to load stand-in sector %d: %v", faultStandIn, err)
	}

	// Expand faults into a map for quick lookups.
	// The faults bitfield should already be a subset of the sectors bitfield.
	sectorCount, err := sectors.Count()
	if err != nil {
		return nil, err
	}
	faultSet, err := faults.AllMap(sectorCount)
	if err != nil {
		return nil, fmt.Errorf("failed to expand faults: %w", err)
	}

	// Load the sector infos, masking out fault sectors with a good one.
	sectorInfos := make([]*SectorOnChainInfo, 0, sectorCount)
	err = sectors.ForEach(func(i uint64) error {
		sector := standInInfo
		faulty := faultSet[i]
		if !faulty {
			sectorOnChain, err := sa.MustGet(abi.SectorNumber(i))
			if err != nil {
				return xerrors.Errorf("failed to load sector %d: %w", i, err)
			}
			sector = sectorOnChain
		}
		sectorInfos = append(sectorInfos, sector)
		return nil
	})
	return sectorInfos, err
}

func selectSectors(sectors []*SectorOnChainInfo, field bitfield.BitField) ([]*SectorOnChainInfo, error) {
	toInclude, err := field.AllMap(uint64(len(sectors)))
	if err != nil {
		return nil, xerrors.Errorf("failed to expand bitfield when selecting sectors: %w", err)
	}

	included := make([]*SectorOnChainInfo, 0, len(toInclude))
	for _, s := range sectors {
		if !toInclude[uint64(s.SectorNumber)] {
			continue
		}
		included = append(included, s)
		delete(toInclude, uint64(s.SectorNumber))
	}
	if len(toInclude) > 0 {
		return nil, xerrors.Errorf("failed to find %d expected sectors", len(toInclude))
	}
	return included, nil
}
