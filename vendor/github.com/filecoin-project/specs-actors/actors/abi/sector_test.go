package abi_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/specs-actors/actors/abi"
)

func TestSectorSizeString(t *testing.T) {
	assert.Equal(t, "0", abi.SectorSize(0).String())
	assert.Equal(t, "1", abi.SectorSize(1).String())
	assert.Equal(t, "1024", abi.SectorSize(1024).String())
	assert.Equal(t, "1234", abi.SectorSize(1234).String())
	assert.Equal(t, "1125899906842624", abi.SectorSize(1125899906842624).String())
}

func TestSectorSizeShortString(t *testing.T) {
	kib := uint64(1024)
	pib := uint64(1125899906842624)

	assert.Equal(t, "0B", abi.SectorSize(0).ShortString())
	assert.Equal(t, "1B", abi.SectorSize(1).ShortString())
	assert.Equal(t, "1023B", abi.SectorSize(1023).ShortString())
	assert.Equal(t, "1KiB", abi.SectorSize(kib).ShortString())
	assert.Equal(t, "1KiB", abi.SectorSize(kib+1).ShortString())   // truncated
	assert.Equal(t, "1KiB", abi.SectorSize(kib*2-1).ShortString()) // truncated
	assert.Equal(t, "2KiB", abi.SectorSize(kib*2).ShortString())
	assert.Equal(t, "2KiB", abi.SectorSize(kib*2+1).ShortString()) // truncated
	assert.Equal(t, "1023KiB", abi.SectorSize(kib*1023).ShortString())
	assert.Equal(t, "1MiB", abi.SectorSize(1048576).ShortString())
	assert.Equal(t, "1GiB", abi.SectorSize(1073741824).ShortString())
	assert.Equal(t, "1TiB", abi.SectorSize(1099511627776).ShortString())
	assert.Equal(t, "1PiB", abi.SectorSize(pib).ShortString())
	assert.Equal(t, "1EiB", abi.SectorSize(pib*kib).ShortString())
	assert.Equal(t, "10EiB", abi.SectorSize(pib*kib*10).ShortString())
}
