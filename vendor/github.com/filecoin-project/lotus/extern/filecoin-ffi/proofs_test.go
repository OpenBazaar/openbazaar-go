package ffi

import (
	"bytes"
	"crypto/rand"
	"github.com/filecoin-project/filecoin-ffi/generated"
	"io"
	"io/ioutil"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	commcid "github.com/filecoin-project/go-fil-commcid"

	"github.com/filecoin-project/specs-actors/actors/abi"

	"github.com/stretchr/testify/require"
)

func TestRegisteredSealProofFunctions(t *testing.T) {
	WorkflowRegisteredSealProofFunctions(newTestingTeeHelper(t))
}

func TestRegisteredPoStProofFunctions(t *testing.T) {
	WorkflowRegisteredPoStProofFunctions(newTestingTeeHelper(t))
}

func TestProofsLifecycle(t *testing.T) {
	WorkflowProofsLifecycle(newTestingTeeHelper(t))
}

func TestGetGPUDevicesDoesNotProduceAnError(t *testing.T) {
	WorkflowGetGPUDevicesDoesNotProduceAnError(newTestingTeeHelper(t))
}

func TestGenerateWinningPoStSectorChallenge(t *testing.T) {
	WorkflowGenerateWinningPoStSectorChallenge(newTestingTeeHelper(t))
}

func TestGenerateWinningPoStSectorChallengeEdgeCase(t *testing.T) {
	WorkflowGenerateWinningPoStSectorChallengeEdgeCase(newTestingTeeHelper(t))
}

func TestJsonMarshalSymmetry(t *testing.T) {
	for i := 0; i < 100; i++ {
		xs := make([]publicSectorInfo, 10)
		for j := 0; j < 10; j++ {
			var x publicSectorInfo
			var commR [32]byte
			_, err := io.ReadFull(rand.Reader, commR[:])
			require.NoError(t, err)

			x.SealedCID = commcid.ReplicaCommitmentV1ToCID(commR[:])

			n, err := rand.Int(rand.Reader, big.NewInt(500))
			require.NoError(t, err)
			x.SectorNum = abi.SectorNumber(n.Uint64())
			xs[j] = x
		}
		toSerialize := newSortedPublicSectorInfo(xs...)

		serialized, err := toSerialize.MarshalJSON()
		require.NoError(t, err)

		var fromSerialized SortedPublicSectorInfo
		err = fromSerialized.UnmarshalJSON(serialized)
		require.NoError(t, err)

		require.Equal(t, toSerialize, fromSerialized)
	}
}

func TestDoesNotExhaustFileDescriptors(t *testing.T) {
	m := 500         // loops
	n := uint64(508) // quantity of piece bytes

	for i := 0; i < m; i++ {
		// create a temporary file over which we'll compute CommP
		file, err := ioutil.TempFile("", "")
		if err != nil {
			panic(err)
		}

		// create a slice of random bytes (represents our piece)
		b := make([]byte, n)

		// load up our byte slice with random bytes
		if _, err = rand.Read(b); err != nil {
			panic(err)
		}

		// write buffer to temp file
		if _, err := bytes.NewBuffer(b).WriteTo(file); err != nil {
			panic(err)
		}

		// seek to beginning of file
		if _, err := file.Seek(0, 0); err != nil {
			panic(err)
		}

		if _, err = GeneratePieceCID(abi.RegisteredSealProof_StackedDrg2KiBV1, file.Name(), abi.UnpaddedPieceSize(n)); err != nil {
			panic(err)
		}

		if err = file.Close(); err != nil {
			panic(err)
		}
	}
}

func newTestingTeeHelper(t *testing.T) *testingTeeHelper {
	return &testingTeeHelper{t: t}
}

type testingTeeHelper struct {
	t *testing.T
}

func (tth *testingTeeHelper) RequireTrue(value bool, msgAndArgs ...interface{}) {
	require.True(tth.t, value, msgAndArgs)
}

func (tth *testingTeeHelper) RequireNoError(err error, msgAndArgs ...interface{}) {
	require.NoError(tth.t, err, msgAndArgs)
}

func (tth *testingTeeHelper) RequireEqual(expected interface{}, actual interface{}, msgAndArgs ...interface{}) {
	require.Equal(tth.t, expected, actual, msgAndArgs)
}

func (tth *testingTeeHelper) AssertNoError(err error, msgAndArgs ...interface{}) bool {
	return assert.NoError(tth.t, err, msgAndArgs)
}

func (tth *testingTeeHelper) AssertEqual(expected, actual interface{}, msgAndArgs ...interface{}) bool {
	return assert.Equal(tth.t, expected, actual, msgAndArgs)
}

func (tth *testingTeeHelper) AssertTrue(value bool, msgAndArgs ...interface{}) bool {
	return assert.True(tth.t, value, msgAndArgs)
}

func TestProofTypes(t *testing.T) {
	assert.EqualValues(t, generated.FilRegisteredPoStProofStackedDrgWinning2KiBV1, abi.RegisteredPoStProof_StackedDrgWinning2KiBV1)
	assert.EqualValues(t, generated.FilRegisteredPoStProofStackedDrgWinning8MiBV1, abi.RegisteredPoStProof_StackedDrgWinning8MiBV1)
	assert.EqualValues(t, generated.FilRegisteredPoStProofStackedDrgWinning512MiBV1, abi.RegisteredPoStProof_StackedDrgWinning512MiBV1)
	assert.EqualValues(t, generated.FilRegisteredPoStProofStackedDrgWinning32GiBV1, abi.RegisteredPoStProof_StackedDrgWinning32GiBV1)
	assert.EqualValues(t, generated.FilRegisteredPoStProofStackedDrgWinning64GiBV1, abi.RegisteredPoStProof_StackedDrgWinning64GiBV1)
	assert.EqualValues(t, generated.FilRegisteredPoStProofStackedDrgWindow2KiBV1, abi.RegisteredPoStProof_StackedDrgWindow2KiBV1)
	assert.EqualValues(t, generated.FilRegisteredPoStProofStackedDrgWindow8MiBV1, abi.RegisteredPoStProof_StackedDrgWindow8MiBV1)
	assert.EqualValues(t, generated.FilRegisteredPoStProofStackedDrgWindow512MiBV1, abi.RegisteredPoStProof_StackedDrgWindow512MiBV1)
	assert.EqualValues(t, generated.FilRegisteredPoStProofStackedDrgWindow32GiBV1, abi.RegisteredPoStProof_StackedDrgWindow32GiBV1)
	assert.EqualValues(t, generated.FilRegisteredPoStProofStackedDrgWindow64GiBV1, abi.RegisteredPoStProof_StackedDrgWindow64GiBV1)

	assert.EqualValues(t, generated.FilRegisteredSealProofStackedDrg2KiBV1, abi.RegisteredSealProof_StackedDrg2KiBV1)
	assert.EqualValues(t, generated.FilRegisteredSealProofStackedDrg8MiBV1, abi.RegisteredSealProof_StackedDrg8MiBV1)
	assert.EqualValues(t, generated.FilRegisteredSealProofStackedDrg512MiBV1, abi.RegisteredSealProof_StackedDrg512MiBV1)
	assert.EqualValues(t, generated.FilRegisteredSealProofStackedDrg32GiBV1, abi.RegisteredSealProof_StackedDrg32GiBV1)
	assert.EqualValues(t, generated.FilRegisteredSealProofStackedDrg64GiBV1, abi.RegisteredSealProof_StackedDrg64GiBV1)
}
