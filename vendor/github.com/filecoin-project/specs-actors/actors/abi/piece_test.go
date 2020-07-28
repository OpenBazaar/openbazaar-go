package abi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPieceSize(t *testing.T) {
	// happy
	require.NoError(t, UnpaddedPieceSize(127).Validate())
	require.NoError(t, UnpaddedPieceSize(1016).Validate())
	require.NoError(t, UnpaddedPieceSize(34091302912).Validate())

	require.NoError(t, PaddedPieceSize(128).Validate())
	require.NoError(t, PaddedPieceSize(1024).Validate())
	require.NoError(t, PaddedPieceSize(34359738368).Validate())

	// convert
	require.Equal(t, PaddedPieceSize(128), UnpaddedPieceSize(127).Padded())
	require.Equal(t, PaddedPieceSize(1024), UnpaddedPieceSize(1016).Padded())
	require.Equal(t, PaddedPieceSize(34359738368), UnpaddedPieceSize(34091302912).Padded())

	require.Equal(t, UnpaddedPieceSize(127), PaddedPieceSize(128).Unpadded())
	require.Equal(t, UnpaddedPieceSize(1016), PaddedPieceSize(1024).Unpadded())
	require.Equal(t, UnpaddedPieceSize(34091302912), PaddedPieceSize(34359738368).Unpadded())

	// swap
	require.NoError(t, UnpaddedPieceSize(127).Padded().Validate())
	require.NoError(t, UnpaddedPieceSize(1016).Padded().Validate())
	require.NoError(t, UnpaddedPieceSize(34091302912).Padded().Validate())

	require.NoError(t, PaddedPieceSize(128).Unpadded().Validate())
	require.NoError(t, PaddedPieceSize(1024).Unpadded().Validate())
	require.NoError(t, PaddedPieceSize(34359738368).Unpadded().Validate())

	// roundtrip
	require.NoError(t, UnpaddedPieceSize(127).Padded().Unpadded().Validate())
	require.NoError(t, UnpaddedPieceSize(1016).Padded().Unpadded().Validate())
	require.NoError(t, UnpaddedPieceSize(34091302912).Padded().Unpadded().Validate())

	require.NoError(t, PaddedPieceSize(128).Unpadded().Padded().Validate())
	require.NoError(t, PaddedPieceSize(1024).Unpadded().Padded().Validate())
	require.NoError(t, PaddedPieceSize(34359738368).Unpadded().Padded().Validate())

	// unhappy
	require.Error(t, UnpaddedPieceSize(9).Validate())
	require.Error(t, UnpaddedPieceSize(128).Validate())
	require.Error(t, UnpaddedPieceSize(99453687).Validate())
	require.Error(t, UnpaddedPieceSize(1016+0x1000000).Validate())

	require.Error(t, PaddedPieceSize(8).Validate())
	require.Error(t, PaddedPieceSize(127).Validate())
	require.Error(t, PaddedPieceSize(99453687).Validate())
	require.Error(t, PaddedPieceSize(0xc00).Validate())
	require.Error(t, PaddedPieceSize(1025).Validate())
}
