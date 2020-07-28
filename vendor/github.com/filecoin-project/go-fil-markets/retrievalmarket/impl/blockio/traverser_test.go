package blockio_test

import (
	"bytes"
	"context"
	"testing"

	blocks "github.com/ipfs/go-block-format"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/blockio"
	"github.com/filecoin-project/go-fil-markets/shared"
	tut "github.com/filecoin-project/go-fil-markets/shared_testutil"
)

func TestTraverser(t *testing.T) {
	ctx := context.Background()
	testdata := tut.NewTestIPLDTree()

	t.Run("traverses correctly", func(t *testing.T) {
		traverser := blockio.NewTraverser(testdata.RootNodeLnk, shared.AllSelector())
		traverser.Start(ctx)
		checkTraverseSequence(ctx, t, traverser, []blocks.Block{
			testdata.RootBlock,
			testdata.LeafAlphaBlock,
			testdata.MiddleMapBlock,
			testdata.LeafAlphaBlock,
			testdata.MiddleListBlock,
			testdata.LeafAlphaBlock,
			testdata.LeafAlphaBlock,
			testdata.LeafBetaBlock,
			testdata.LeafAlphaBlock,
		})
	})

}

func checkTraverseSequence(ctx context.Context, t *testing.T, traverser *blockio.Traverser, expectedBlks []blocks.Block) {
	for _, blk := range expectedBlks {
		require.False(t, traverser.IsComplete(ctx))
		lnk, _ := traverser.CurrentRequest(ctx)
		require.Equal(t, lnk.(cidlink.Link).Cid, blk.Cid())
		err := traverser.Advance(ctx, bytes.NewBuffer(blk.RawData()))
		require.NoError(t, err)
	}
	require.True(t, traverser.IsComplete(ctx))
}
