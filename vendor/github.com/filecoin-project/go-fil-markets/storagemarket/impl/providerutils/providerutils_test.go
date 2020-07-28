package providerutils_test

import (
	"bytes"
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car"
	"github.com/ipld/go-ipld-prime"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/traversal/selector/builder"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/blockrecorder"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/providerutils"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
)

func TestVerifyProposal(t *testing.T) {
	tests := map[string]struct {
		proposal  market.ClientDealProposal
		verifier  providerutils.VerifyFunc
		shouldErr bool
	}{
		"successful verification": {
			proposal: *shared_testutil.MakeTestClientDealProposal(),
			verifier: func(context.Context, crypto.Signature, address.Address, []byte, shared.TipSetToken) (bool, error) {
				return true, nil
			},
			shouldErr: false,
		},
		"bad proposal": {
			proposal: market.ClientDealProposal{
				Proposal:        market.DealProposal{},
				ClientSignature: *shared_testutil.MakeTestSignature(),
			},
			verifier: func(context.Context, crypto.Signature, address.Address, []byte, shared.TipSetToken) (bool, error) {
				return true, nil
			},
			shouldErr: true,
		},
		"verification fails": {
			proposal: *shared_testutil.MakeTestClientDealProposal(),
			verifier: func(context.Context, crypto.Signature, address.Address, []byte, shared.TipSetToken) (bool, error) {
				return false, nil
			},
			shouldErr: true,
		},
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			err := providerutils.VerifyProposal(context.Background(), data.proposal, shared.TipSetToken{}, data.verifier)
			require.Equal(t, err != nil, data.shouldErr)
		})
	}
}

func TestSignMinerData(t *testing.T) {
	ctx := context.Background()
	successLookup := func(context.Context, address.Address, shared.TipSetToken) (address.Address, error) {
		return address.TestAddress2, nil
	}
	successSign := func(context.Context, address.Address, []byte) (*crypto.Signature, error) {
		return shared_testutil.MakeTestSignature(), nil
	}
	tests := map[string]struct {
		data         interface{}
		workerLookup providerutils.WorkerLookupFunc
		signBytes    providerutils.SignFunc
		shouldErr    bool
	}{
		"succeeds": {
			data:         shared_testutil.MakeTestStorageAsk(),
			workerLookup: successLookup,
			signBytes:    successSign,
			shouldErr:    false,
		},
		"cbor dump errors": {
			data:         &network.Response{},
			workerLookup: successLookup,
			signBytes:    successSign,
			shouldErr:    true,
		},
		"worker lookup errors": {
			data: shared_testutil.MakeTestStorageAsk(),
			workerLookup: func(context.Context, address.Address, shared.TipSetToken) (address.Address, error) {
				return address.Undef, errors.New("Something went wrong")
			},
			signBytes: successSign,
			shouldErr: true,
		},
		"signing errors": {
			data:         shared_testutil.MakeTestStorageAsk(),
			workerLookup: successLookup,
			signBytes: func(context.Context, address.Address, []byte) (*crypto.Signature, error) {
				return nil, errors.New("something went wrong")
			},
			shouldErr: true,
		},
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := providerutils.SignMinerData(ctx, data.data, address.TestAddress, shared.TipSetToken{}, data.workerLookup, data.signBytes)
			require.Equal(t, err != nil, data.shouldErr)
		})
	}
}

func TestCommPGenerationWithMetadata(t *testing.T) {
	tempFilePath := filestore.Path("applesauce.jpg")
	tempFile := shared_testutil.NewTestFile(shared_testutil.TestFileParams{Path: tempFilePath})
	payloadCid := shared_testutil.GenerateCids(1)[0]
	ssb := builder.NewSelectorSpecBuilder(basicnode.Style.Any)
	selector := ssb.ExploreAll(ssb.Matcher()).Node()
	proofType := abi.RegisteredSealProof_StackedDrg2KiBV1
	pieceCid := shared_testutil.GenerateCids(1)[0]
	piecePath := filestore.Path("apiece.jpg")
	pieceSize := abi.UnpaddedPieceSize(rand.Uint64())
	testCases := map[string]struct {
		fileStoreParams      shared_testutil.TestFileStoreParams
		commPErr             error
		expectedPieceCid     cid.Cid
		expectedPiecePath    filestore.Path
		expectedMetadataPath filestore.Path
		shouldErr            bool
	}{
		"success": {
			fileStoreParams: shared_testutil.TestFileStoreParams{
				AvailableTempFiles: []filestore.File{tempFile},
			},
			expectedPieceCid:     pieceCid,
			expectedPiecePath:    piecePath,
			expectedMetadataPath: tempFilePath,
			shouldErr:            false,
		},
		"tempfile creations fails": {
			fileStoreParams: shared_testutil.TestFileStoreParams{},
			shouldErr:       true,
		},
		"commP generation fails": {
			fileStoreParams: shared_testutil.TestFileStoreParams{
				AvailableTempFiles: []filestore.File{tempFile},
				ExpectedDeletions:  []filestore.Path{tempFile.Path()},
			},
			commPErr:  errors.New("Could not generate commP"),
			shouldErr: true,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			fcp := &fakeCommPGenerator{pieceCid, piecePath, pieceSize, testCase.commPErr}
			fs := shared_testutil.NewTestFileStore(testCase.fileStoreParams)
			resultPieceCid, resultPiecePath, resultMetadataPath, resultErr := providerutils.GeneratePieceCommitmentWithMetadata(
				fs, fcp.GenerateCommPToFile, proofType, payloadCid, selector)
			require.Equal(t, resultPieceCid, testCase.expectedPieceCid)
			require.Equal(t, resultPiecePath, testCase.expectedPiecePath)
			require.Equal(t, resultMetadataPath, testCase.expectedMetadataPath)
			if testCase.shouldErr {
				require.Error(t, resultErr)
			} else {
				require.NoError(t, resultErr)
			}
			fs.VerifyExpectations(t)
		})
	}
}

type fakeCommPGenerator struct {
	pieceCid cid.Cid
	path     filestore.Path
	size     abi.UnpaddedPieceSize
	err      error
}

func (fcp *fakeCommPGenerator) GenerateCommPToFile(abi.RegisteredSealProof, cid.Cid, ipld.Node, ...car.OnNewCarBlockFunc) (cid.Cid, filestore.Path, abi.UnpaddedPieceSize, error) {
	return fcp.pieceCid, fcp.path, fcp.size, fcp.err
}

func TestLoadBlockLocations(t *testing.T) {
	testData := shared_testutil.NewTestIPLDTree()

	carBuf := new(bytes.Buffer)
	blockLocationBuf := new(bytes.Buffer)
	err := testData.DumpToCar(carBuf, blockrecorder.RecordEachBlockTo(blockLocationBuf))
	require.NoError(t, err)
	validPath := filestore.Path("valid.data")
	validFile := shared_testutil.NewTestFile(shared_testutil.TestFileParams{
		Buffer: blockLocationBuf,
		Path:   validPath,
	})
	missingPath := filestore.Path("missing.data")
	invalidPath := filestore.Path("invalid.data")
	invalidData := make([]byte, 512)
	_, _ = rand.Read(invalidData)
	invalidFile := shared_testutil.NewTestFile(shared_testutil.TestFileParams{
		Buffer: bytes.NewBuffer(invalidData),
		Path:   invalidPath,
	})
	fs := shared_testutil.NewTestFileStore(shared_testutil.TestFileStoreParams{
		Files:         []filestore.File{validFile, invalidFile},
		ExpectedOpens: []filestore.Path{validPath, invalidPath},
	})
	testCases := map[string]struct {
		path         filestore.Path
		shouldErr    bool
		expectedCids []cid.Cid
	}{
		"valid data": {
			path:      validPath,
			shouldErr: false,
			expectedCids: []cid.Cid{
				testData.LeafAlphaBlock.Cid(),
				testData.LeafBetaBlock.Cid(),
				testData.MiddleListBlock.Cid(),
				testData.MiddleMapBlock.Cid(),
				testData.RootBlock.Cid(),
			},
		},
		"missing data": {
			path:      missingPath,
			shouldErr: true,
		},
		"invalid data": {
			path:      invalidPath,
			shouldErr: true,
		},
	}
	for testCase, data := range testCases {
		t.Run(testCase, func(t *testing.T) {
			results, err := providerutils.LoadBlockLocations(fs, data.path)
			if data.shouldErr {
				require.Error(t, err)
				require.Nil(t, results)
			} else {
				require.NoError(t, err)
				for _, c := range data.expectedCids {
					_, ok := results[c]
					require.True(t, ok)
				}
			}
		})
	}
	fs.VerifyExpectations(t)
}
