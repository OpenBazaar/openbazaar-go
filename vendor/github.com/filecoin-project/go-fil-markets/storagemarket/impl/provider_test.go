package storageimpl_test

import (
	"testing"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/stretchr/testify/assert"

	storageimpl "github.com/filecoin-project/go-fil-markets/storagemarket/impl"
)

func TestConfigure(t *testing.T) {
	p := &storageimpl.Provider{}

	assert.False(t, p.UniversalRetrievalEnabled())
	assert.Equal(t, abi.ChainEpoch(0), p.DealAcceptanceBuffer())

	p.Configure(
		storageimpl.EnableUniversalRetrieval(),
		storageimpl.DealAcceptanceBuffer(abi.ChainEpoch(123)),
	)

	assert.True(t, p.UniversalRetrievalEnabled())
	assert.Equal(t, abi.ChainEpoch(123), p.DealAcceptanceBuffer())
}
