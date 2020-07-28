package miner

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpirations(t *testing.T) {
	sectors := []*SectorOnChainInfo{{
		Expiration:   0,
		SectorNumber: 1,
	}, {
		Expiration:   0,
		SectorNumber: 2,
	}, {
		Expiration:   2,
		SectorNumber: 3,
	}, {
		Expiration:   0,
		SectorNumber: 4,
	}}
	result := groupSectorsByExpiration(sectors)
	expected := []sectorEpochSet{{
		epoch:   0,
		sectors: []uint64{1, 2, 4},
	}, {
		epoch:   2,
		sectors: []uint64{3},
	}}
	require.Equal(t, expected, result)
}

func TestExpirationsEmpty(t *testing.T) {
	sectors := []*SectorOnChainInfo{}
	result := groupSectorsByExpiration(sectors)
	expected := []sectorEpochSet{}
	require.Equal(t, expected, result)
}
