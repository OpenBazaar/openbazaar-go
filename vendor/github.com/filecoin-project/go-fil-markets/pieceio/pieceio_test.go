package pieceio_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/filecoin-project/sector-storage/ffiwrapper"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-cid"
	dag "github.com/ipfs/go-merkledag"
	dstest "github.com/ipfs/go-merkledag/test"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/ipld/go-ipld-prime/traversal/selector/builder"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-fil-markets/filestore"
	fsmocks "github.com/filecoin-project/go-fil-markets/filestore/mocks"
	"github.com/filecoin-project/go-fil-markets/pieceio"
	"github.com/filecoin-project/go-fil-markets/pieceio/cario"
	pmocks "github.com/filecoin-project/go-fil-markets/pieceio/mocks"
)

func Test_ThereAndBackAgain(t *testing.T) {
	tempDir := filestore.OsPath("./tempDir")
	cio := cario.NewCarIO()

	store, err := filestore.NewLocalFileStore(tempDir)
	require.NoError(t, err)

	sourceBserv := dstest.Bserv()
	sourceBs := sourceBserv.Blockstore()

	pio := pieceio.NewPieceIOWithStore(cio, store, sourceBs)
	require.NoError(t, err)

	dserv := dag.NewDAGService(sourceBserv)
	a := dag.NewRawNode([]byte("aaaa"))
	b := dag.NewRawNode([]byte("bbbb"))
	c := dag.NewRawNode([]byte("cccc"))

	nd1 := &dag.ProtoNode{}
	_ = nd1.AddNodeLink("cat", a)

	nd2 := &dag.ProtoNode{}
	_ = nd2.AddNodeLink("first", nd1)
	_ = nd2.AddNodeLink("dog", b)

	nd3 := &dag.ProtoNode{}
	_ = nd3.AddNodeLink("second", nd2)
	_ = nd3.AddNodeLink("bear", c)

	ctx := context.Background()
	_ = dserv.Add(ctx, a)
	_ = dserv.Add(ctx, b)
	_ = dserv.Add(ctx, c)
	_ = dserv.Add(ctx, nd1)
	_ = dserv.Add(ctx, nd2)
	_ = dserv.Add(ctx, nd3)

	ssb := builder.NewSelectorSpecBuilder(basicnode.Style.Any)
	node := ssb.ExploreFields(func(efsb builder.ExploreFieldsSpecBuilder) {
		efsb.Insert("Links",
			ssb.ExploreIndex(1, ssb.ExploreRecursive(selector.RecursionLimitNone(), ssb.ExploreAll(ssb.ExploreRecursiveEdge()))))
	}).Node()

	pcid, tmpPath, _, err := pio.GeneratePieceCommitmentToFile(abi.RegisteredSealProof_StackedDrg2KiBV1, nd3.Cid(), node)
	require.NoError(t, err)
	tmpFile, err := store.Open(tmpPath)
	require.NoError(t, err)
	defer func() {
		deferErr := tmpFile.Close()
		require.NoError(t, deferErr)
		deferErr = store.Delete(tmpFile.Path())
		require.NoError(t, deferErr)
	}()
	require.NotEqual(t, pcid, cid.Undef)
	bufSize := int64(16) // small buffer to illustrate the logic
	buf := make([]byte, bufSize)
	var readErr error
	padStart := int64(-1)
	loops := int64(-1)
	read := 0
	skipped, err := tmpFile.Seek(tmpFile.Size()/2, io.SeekStart)
	require.NoError(t, err)
	for readErr == nil {
		loops++
		read, readErr = tmpFile.Read(buf)
		for idx := int64(0); idx < int64(read); idx++ {
			if buf[idx] == 0 {
				if padStart == -1 {
					padStart = skipped + loops*bufSize + idx
				}
			} else {
				padStart = -1
			}
		}
	}
	_, err = tmpFile.Seek(0, io.SeekStart)
	require.NoError(t, err)

	var reader io.Reader
	if padStart != -1 {
		reader = io.LimitReader(tmpFile, padStart)
	} else {
		reader = tmpFile
	}

	id, err := pio.ReadPiece(reader)
	require.NoError(t, err)
	require.Equal(t, nd3.Cid(), id)
}

func Test_StoreRestoreMemoryBuffer(t *testing.T) {
	tempDir := filestore.OsPath("./tempDir")
	cio := cario.NewCarIO()

	store, err := filestore.NewLocalFileStore(tempDir)
	require.NoError(t, err)

	sourceBserv := dstest.Bserv()
	sourceBs := sourceBserv.Blockstore()
	pio := pieceio.NewPieceIOWithStore(cio, store, sourceBs)

	dserv := dag.NewDAGService(sourceBserv)
	a := dag.NewRawNode([]byte("aaaa"))
	b := dag.NewRawNode([]byte("bbbb"))
	c := dag.NewRawNode([]byte("cccc"))

	nd1 := &dag.ProtoNode{}
	_ = nd1.AddNodeLink("cat", a)

	nd2 := &dag.ProtoNode{}
	_ = nd2.AddNodeLink("first", nd1)
	_ = nd2.AddNodeLink("dog", b)

	nd3 := &dag.ProtoNode{}
	_ = nd3.AddNodeLink("second", nd2)
	_ = nd3.AddNodeLink("bear", c)

	ctx := context.Background()
	_ = dserv.Add(ctx, a)
	_ = dserv.Add(ctx, b)
	_ = dserv.Add(ctx, c)
	_ = dserv.Add(ctx, nd1)
	_ = dserv.Add(ctx, nd2)
	_ = dserv.Add(ctx, nd3)

	ssb := builder.NewSelectorSpecBuilder(basicnode.Style.Any)
	node := ssb.ExploreFields(func(efsb builder.ExploreFieldsSpecBuilder) {
		efsb.Insert("Links",
			ssb.ExploreIndex(1, ssb.ExploreRecursive(selector.RecursionLimitNone(), ssb.ExploreAll(ssb.ExploreRecursiveEdge()))))
	}).Node()

	commitment, tmpPath, paddedSize, err := pio.GeneratePieceCommitmentToFile(abi.RegisteredSealProof_StackedDrg2KiBV1, nd3.Cid(), node)
	require.NoError(t, err)
	tmpFile, err := store.Open(tmpPath)
	require.NoError(t, err)
	defer func() {
		deferErr := tmpFile.Close()
		require.NoError(t, deferErr)
		deferErr = store.Delete(tmpFile.Path())
		require.NoError(t, deferErr)
	}()

	_, err = tmpFile.Seek(0, io.SeekStart)
	require.NoError(t, err)

	require.NotEqual(t, commitment, cid.Undef)
	buf := make([]byte, paddedSize)
	_, err = tmpFile.Read(buf)
	require.NoError(t, err)
	buffer := bytes.NewBuffer(buf)
	secondCommitment, err := ffiwrapper.GeneratePieceCIDFromFile(abi.RegisteredSealProof_StackedDrg2KiBV1, buffer, paddedSize)
	require.NoError(t, err)
	require.Equal(t, commitment, secondCommitment)
}

func Test_PieceCommitmentEquivalenceMemoryFile(t *testing.T) {
	tempDir := filestore.OsPath("./tempDir")
	cio := cario.NewCarIO()

	store, err := filestore.NewLocalFileStore(tempDir)
	require.NoError(t, err)

	sourceBserv := dstest.Bserv()
	sourceBs := sourceBserv.Blockstore()
	pio := pieceio.NewPieceIOWithStore(cio, store, sourceBs)

	dserv := dag.NewDAGService(sourceBserv)
	a := dag.NewRawNode([]byte("aaaa"))
	b := dag.NewRawNode([]byte("bbbb"))
	c := dag.NewRawNode([]byte("cccc"))

	nd1 := &dag.ProtoNode{}
	_ = nd1.AddNodeLink("cat", a)

	nd2 := &dag.ProtoNode{}
	_ = nd2.AddNodeLink("first", nd1)
	_ = nd2.AddNodeLink("dog", b)

	nd3 := &dag.ProtoNode{}
	_ = nd3.AddNodeLink("second", nd2)
	_ = nd3.AddNodeLink("bear", c)

	ctx := context.Background()
	_ = dserv.Add(ctx, a)
	_ = dserv.Add(ctx, b)
	_ = dserv.Add(ctx, c)
	_ = dserv.Add(ctx, nd1)
	_ = dserv.Add(ctx, nd2)
	_ = dserv.Add(ctx, nd3)

	ssb := builder.NewSelectorSpecBuilder(basicnode.Style.Any)
	node := ssb.ExploreFields(func(efsb builder.ExploreFieldsSpecBuilder) {
		efsb.Insert("Links",
			ssb.ExploreIndex(1, ssb.ExploreRecursive(selector.RecursionLimitNone(), ssb.ExploreAll(ssb.ExploreRecursiveEdge()))))
	}).Node()

	fcommitment, tmpPath, fpaddedSize, ferr := pio.GeneratePieceCommitmentToFile(abi.RegisteredSealProof_StackedDrg2KiBV1, nd3.Cid(), node)
	defer func() {
		deferErr := store.Delete(tmpPath)
		require.NoError(t, deferErr)
	}()

	mcommitment, mpaddedSize, merr := pio.GeneratePieceCommitment(abi.RegisteredSealProof_StackedDrg2KiBV1, nd3.Cid(), node)
	require.Equal(t, fcommitment, mcommitment)
	require.Equal(t, fpaddedSize, mpaddedSize)
	require.Equal(t, ferr, merr)
	require.NoError(t, ferr)
	require.NoError(t, merr)
}

func Test_Failures(t *testing.T) {
	sourceBserv := dstest.Bserv()
	sourceBs := sourceBserv.Blockstore()
	dserv := dag.NewDAGService(sourceBserv)
	a := dag.NewRawNode([]byte("aaaa"))
	b := dag.NewRawNode([]byte("bbbb"))
	c := dag.NewRawNode([]byte("cccc"))

	nd1 := &dag.ProtoNode{}
	_ = nd1.AddNodeLink("cat", a)

	nd2 := &dag.ProtoNode{}
	_ = nd2.AddNodeLink("first", nd1)
	_ = nd2.AddNodeLink("dog", b)

	nd3 := &dag.ProtoNode{}
	_ = nd3.AddNodeLink("second", nd2)
	_ = nd3.AddNodeLink("bear", c)

	ctx := context.Background()
	_ = dserv.Add(ctx, a)
	_ = dserv.Add(ctx, b)
	_ = dserv.Add(ctx, c)
	_ = dserv.Add(ctx, nd1)
	_ = dserv.Add(ctx, nd2)
	_ = dserv.Add(ctx, nd3)

	ssb := builder.NewSelectorSpecBuilder(basicnode.Style.Any)
	node := ssb.ExploreFields(func(efsb builder.ExploreFieldsSpecBuilder) {
		efsb.Insert("Links",
			ssb.ExploreIndex(1, ssb.ExploreRecursive(selector.RecursionLimitNone(), ssb.ExploreAll(ssb.ExploreRecursiveEdge()))))
	}).Node()

	t.Run("create temp file fails", func(t *testing.T) {
		fsmock := fsmocks.FileStore{}
		fsmock.On("CreateTemp").Return(nil, fmt.Errorf("Failed"))
		pio := pieceio.NewPieceIOWithStore(nil, &fsmock, sourceBs)
		_, _, _, err := pio.GeneratePieceCommitmentToFile(abi.RegisteredSealProof_StackedDrg2KiBV1, nd3.Cid(), node)
		require.Error(t, err)
	})
	t.Run("write CAR fails", func(t *testing.T) {
		tempDir := filestore.OsPath("./tempDir")
		store, err := filestore.NewLocalFileStore(tempDir)
		require.NoError(t, err)

		ciomock := pmocks.CarIO{}
		any := mock.Anything
		ciomock.On("WriteCar", any, any, any, any, any).Return(fmt.Errorf("failed to write car"))
		pio := pieceio.NewPieceIOWithStore(&ciomock, store, sourceBs)
		_, _, _, err = pio.GeneratePieceCommitmentToFile(abi.RegisteredSealProof_StackedDrg2KiBV1, nd3.Cid(), node)
		require.Error(t, err)
	})
	t.Run("prepare CAR fails", func(t *testing.T) {

		ciomock := pmocks.CarIO{}
		any := mock.Anything
		ciomock.On("PrepareCar", any, any, any, any).Return(nil, fmt.Errorf("failed to prepare car"))
		pio := pieceio.NewPieceIO(&ciomock, sourceBs)
		_, _, err := pio.GeneratePieceCommitment(abi.RegisteredSealProof_StackedDrg2KiBV1, nd3.Cid(), node)
		require.Error(t, err)
	})
	t.Run("PreparedCard dump operation fails", func(t *testing.T) {
		preparedCarMock := pmocks.PreparedCar{}
		ciomock := pmocks.CarIO{}
		any := mock.Anything
		ciomock.On("PrepareCar", any, any, any, any).Return(&preparedCarMock, nil)
		preparedCarMock.On("Size").Return(uint64(1000))
		preparedCarMock.On("Dump", any).Return(fmt.Errorf("failed to write car"))
		pio := pieceio.NewPieceIO(&ciomock, sourceBs)
		_, _, err := pio.GeneratePieceCommitment(abi.RegisteredSealProof_StackedDrg2KiBV1, nd3.Cid(), node)
		require.Error(t, err)
	})
	t.Run("seek fails", func(t *testing.T) {
		cio := cario.NewCarIO()

		fsmock := fsmocks.FileStore{}
		mockfile := fsmocks.File{}

		fsmock.On("CreateTemp").Return(&mockfile, nil).Once()
		fsmock.On("Delete", mock.Anything).Return(nil).Once()

		counter := 0
		size := 0
		mockfile.On("Write", mock.Anything).Run(func(args mock.Arguments) {
			arg := args[0]
			buf := arg.([]byte)
			size := len(buf)
			counter += size
		}).Return(size, nil).Times(17)
		mockfile.On("Size").Return(int64(484))
		mockfile.On("Write", mock.Anything).Return(24, nil).Once()
		mockfile.On("Close").Return(nil).Once()
		mockfile.On("Path").Return(filestore.Path("mock")).Once()
		mockfile.On("Seek", mock.Anything, mock.Anything).Return(int64(0), fmt.Errorf("seek failed"))

		pio := pieceio.NewPieceIOWithStore(cio, &fsmock, sourceBs)
		_, _, _, err := pio.GeneratePieceCommitmentToFile(abi.RegisteredSealProof_StackedDrg2KiBV1, nd3.Cid(), node)
		require.Error(t, err)
	})
}
