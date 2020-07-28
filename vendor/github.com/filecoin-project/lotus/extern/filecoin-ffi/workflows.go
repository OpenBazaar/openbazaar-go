//+build cgo

package ffi

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-cid"
)

func WorkflowProofsLifecycle(t TestHelper) {
	minerID := abi.ActorID(42)
	randomness := [32]byte{9, 9, 9}
	sealProofType := abi.RegisteredSealProof_StackedDrg2KiBV1
	winningPostProofType := abi.RegisteredPoStProof_StackedDrgWinning2KiBV1
	sectorNum := abi.SectorNumber(42)

	ticket := abi.SealRandomness{5, 4, 2}

	seed := abi.InteractiveSealRandomness{7, 4, 2}

	// initialize a sector builder
	metadataDir := requireTempDirPath(t, "metadata")
	defer os.RemoveAll(metadataDir)

	sealedSectorsDir := requireTempDirPath(t, "sealed-sectors")
	defer os.RemoveAll(sealedSectorsDir)

	stagedSectorsDir := requireTempDirPath(t, "staged-sectors")
	defer os.RemoveAll(stagedSectorsDir)

	sectorCacheRootDir := requireTempDirPath(t, "sector-cache-root-dir")
	defer os.RemoveAll(sectorCacheRootDir)

	sectorCacheDirPath := requireTempDirPath(t, "sector-cache-dir")
	defer os.RemoveAll(sectorCacheDirPath)

	fauxSectorCacheDirPath := requireTempDirPath(t, "faux-sector-cache-dir")
	defer os.RemoveAll(fauxSectorCacheDirPath)

	stagedSectorFile := requireTempFile(t, bytes.NewReader([]byte{}), 0)
	defer stagedSectorFile.Close()

	sealedSectorFile := requireTempFile(t, bytes.NewReader([]byte{}), 0)
	defer sealedSectorFile.Close()

	fauxSealedSectorFile := requireTempFile(t, bytes.NewReader([]byte{}), 0)
	defer fauxSealedSectorFile.Close()

	unsealOutputFileA := requireTempFile(t, bytes.NewReader([]byte{}), 0)
	defer unsealOutputFileA.Close()

	unsealOutputFileB := requireTempFile(t, bytes.NewReader([]byte{}), 0)
	defer unsealOutputFileB.Close()

	unsealOutputFileC := requireTempFile(t, bytes.NewReader([]byte{}), 0)
	defer unsealOutputFileC.Close()

	unsealOutputFileD := requireTempFile(t, bytes.NewReader([]byte{}), 0)
	defer unsealOutputFileD.Close()

	// some rando bytes
	someBytes := make([]byte, abi.PaddedPieceSize(2048).Unpadded())
	_, err := io.ReadFull(rand.Reader, someBytes)
	t.RequireNoError(err)

	// write first piece
	pieceFileA := requireTempFile(t, bytes.NewReader(someBytes[0:127]), 127)

	pieceCIDA, err := GeneratePieceCIDFromFile(sealProofType, pieceFileA, 127)
	t.RequireNoError(err)

	// seek back to head (generating piece commitment moves offset)
	_, err = pieceFileA.Seek(0, 0)
	t.RequireNoError(err)

	// write the first piece using the alignment-free function
	n, pieceCID, err := WriteWithoutAlignment(sealProofType, pieceFileA, 127, stagedSectorFile)
	t.RequireNoError(err)
	t.AssertEqual(int(n), 127)
	t.AssertTrue(pieceCID.Equals(pieceCIDA))

	// write second piece + alignment
	t.RequireNoError(err)
	pieceFileB := requireTempFile(t, bytes.NewReader(someBytes[0:1016]), 1016)

	pieceCIDB, err := GeneratePieceCIDFromFile(sealProofType, pieceFileB, 1016)
	t.RequireNoError(err)

	// seek back to head
	_, err = pieceFileB.Seek(0, 0)
	t.RequireNoError(err)

	// second piece relies on the alignment-computing version
	left, tot, pieceCID, err := WriteWithAlignment(sealProofType, pieceFileB, 1016, stagedSectorFile, []abi.UnpaddedPieceSize{127})
	t.RequireNoError(err)
	t.AssertEqual(889, int(left))
	t.AssertEqual(1905, int(tot))
	t.AssertTrue(pieceCID.Equals(pieceCIDB))

	publicPieces := []abi.PieceInfo{{
		Size:     abi.UnpaddedPieceSize(127).Padded(),
		PieceCID: pieceCIDA,
	}, {
		Size:     abi.UnpaddedPieceSize(1016).Padded(),
		PieceCID: pieceCIDB,
	}}

	preGeneratedUnsealedCID, err := GenerateUnsealedCID(sealProofType, publicPieces)
	t.RequireNoError(err)

	// pre-commit the sector
	sealPreCommitPhase1Output, err := SealPreCommitPhase1(sealProofType, sectorCacheDirPath, stagedSectorFile.Name(), sealedSectorFile.Name(), sectorNum, minerID, ticket, publicPieces)
	t.RequireNoError(err)

	sealedCID, unsealedCID, err := SealPreCommitPhase2(sealPreCommitPhase1Output, sectorCacheDirPath, sealedSectorFile.Name())
	t.RequireNoError(err)

	t.AssertTrue(unsealedCID.Equals(preGeneratedUnsealedCID), "prover and verifier should agree on data commitment")

	// commit the sector
	sealCommitPhase1Output, err := SealCommitPhase1(sealProofType, sealedCID, unsealedCID, sectorCacheDirPath, sealedSectorFile.Name(), sectorNum, minerID, ticket, seed, publicPieces)
	t.RequireNoError(err)

	proof, err := SealCommitPhase2(sealCommitPhase1Output, sectorNum, minerID)
	t.RequireNoError(err)

	// verify the 'ole proofy
	isValid, err := VerifySeal(abi.SealVerifyInfo{
		SectorID: abi.SectorID{
			Miner:  minerID,
			Number: sectorNum,
		},
		SealedCID:             sealedCID,
		SealProof:             sealProofType,
		Proof:                 proof,
		DealIDs:               []abi.DealID{},
		Randomness:            ticket,
		InteractiveRandomness: seed,
		UnsealedCID:           unsealedCID,
	})
	t.RequireNoError(err)
	t.RequireTrue(isValid, "proof wasn't valid")

	// unseal the entire sector and verify that things went as we planned
	_, err = sealedSectorFile.Seek(0, 0)
	t.RequireNoError(err)
	t.RequireNoError(Unseal(sealProofType, sectorCacheDirPath, sealedSectorFile, unsealOutputFileA, sectorNum, minerID, ticket, unsealedCID))
	_, err = unsealOutputFileA.Seek(0, 0)
	t.RequireNoError(err)
	contents, err := ioutil.ReadFile(unsealOutputFileA.Name())
	t.RequireNoError(err)

	// unsealed sector includes a bunch of alignment NUL-bytes
	alignment := make([]byte, 889)

	// verify that we unsealed what we expected to unseal
	t.AssertTrue(bytes.Equal(someBytes[0:127], contents[0:127]), "bytes aren't equal")
	t.AssertTrue(bytes.Equal(alignment, contents[127:1016]), "bytes aren't equal")
	t.AssertTrue(bytes.Equal(someBytes[0:1016], contents[1016:2032]), "bytes aren't equal")

	// unseal just the first piece
	_, err = sealedSectorFile.Seek(0, 0)
	t.RequireNoError(err)
	err = UnsealRange(sealProofType, sectorCacheDirPath, sealedSectorFile, unsealOutputFileB, sectorNum, minerID, ticket, unsealedCID, 0, 127)
	t.RequireNoError(err)
	_, err = unsealOutputFileB.Seek(0, 0)
	t.RequireNoError(err)
	contentsB, err := ioutil.ReadFile(unsealOutputFileB.Name())
	t.RequireNoError(err)
	t.AssertEqual(127, len(contentsB))
	t.AssertTrue(bytes.Equal(someBytes[0:127], contentsB[0:127]), "bytes aren't equal")

	// unseal just the second piece
	_, err = sealedSectorFile.Seek(0, 0)
	t.RequireNoError(err)
	err = UnsealRange(sealProofType, sectorCacheDirPath, sealedSectorFile, unsealOutputFileC, sectorNum, minerID, ticket, unsealedCID, 1016, 1016)
	t.RequireNoError(err)
	_, err = unsealOutputFileC.Seek(0, 0)
	t.RequireNoError(err)
	contentsC, err := ioutil.ReadFile(unsealOutputFileC.Name())
	t.RequireNoError(err)
	t.AssertEqual(1016, len(contentsC))
	t.AssertTrue(bytes.Equal(someBytes[0:1016], contentsC[0:1016]), "bytes aren't equal")

	// verify that the sector builder owns no sealed sectors
	var sealedSectorPaths []string
	t.RequireNoError(filepath.Walk(sealedSectorsDir, visit(&sealedSectorPaths)))
	t.AssertEqual(1, len(sealedSectorPaths), sealedSectorPaths)

	// no sector cache dirs, either
	var sectorCacheDirPaths []string
	t.RequireNoError(filepath.Walk(sectorCacheRootDir, visit(&sectorCacheDirPaths)))
	t.AssertEqual(1, len(sectorCacheDirPaths), sectorCacheDirPaths)

	// run the FauxRep routine, for good measure
	fauxSectorCID, err := FauxRep(sealProofType, fauxSectorCacheDirPath, fauxSealedSectorFile.Name())
	t.RequireNoError(err, "FauxRep produced an error")
	t.RequireTrue(!cid.Undef.Equals(fauxSectorCID), "faux sector CID shouldn't be undefined")

	// generate a PoSt over the proving set before importing, just to exercise
	// the new API
	privateInfo := NewSortedPrivateSectorInfo(PrivateSectorInfo{
		SectorInfo: abi.SectorInfo{
			SectorNumber: sectorNum,
			SealedCID:    sealedCID,
		},
		CacheDirPath:     sectorCacheDirPath,
		PoStProofType:    winningPostProofType,
		SealedSectorPath: sealedSectorFile.Name(),
	})

	provingSet := []abi.SectorInfo{{
		SealProof:    sealProofType,
		SectorNumber: sectorNum,
		SealedCID:    sealedCID,
	}}

	// figure out which sectors have been challenged
	indicesInProvingSet, err := GenerateWinningPoStSectorChallenge(winningPostProofType, minerID, randomness[:], uint64(len(provingSet)))
	t.RequireNoError(err)

	var challengedSectors []abi.SectorInfo
	for idx := range indicesInProvingSet {
		challengedSectors = append(challengedSectors, provingSet[indicesInProvingSet[idx]])
	}

	proofs, err := GenerateWinningPoSt(minerID, privateInfo, randomness[:])
	t.RequireNoError(err)

	isValid, err = VerifyWinningPoSt(abi.WinningPoStVerifyInfo{
		Randomness:        randomness[:],
		Proofs:            proofs,
		ChallengedSectors: challengedSectors,
		Prover:            minerID,
	})
	t.RequireNoError(err)
	t.AssertTrue(isValid, "VerifyWinningPoSt rejected the (standalone) proof as invalid")
}

func WorkflowGetGPUDevicesDoesNotProduceAnError(t TestHelper) {
	devices, err := GetGPUDevices()
	t.RequireNoError(err)
	fmt.Printf("devices: %+v\n", devices) // clutters up test output, but useful
}

func WorkflowRegisteredSealProofFunctions(t TestHelper) {
	sealTypes := []abi.RegisteredSealProof{
		abi.RegisteredSealProof_StackedDrg2KiBV1,
		abi.RegisteredSealProof_StackedDrg8MiBV1,
		abi.RegisteredSealProof_StackedDrg512MiBV1,
		abi.RegisteredSealProof_StackedDrg32GiBV1,
		abi.RegisteredSealProof_StackedDrg64GiBV1,
	}

	for _, st := range sealTypes {
		v, err := GetSealVersion(st)
		t.AssertNoError(err)
		t.AssertTrue(len(v) > 0)
	}
}

func WorkflowRegisteredPoStProofFunctions(t TestHelper) {
	postTypes := []abi.RegisteredPoStProof{
		abi.RegisteredPoStProof_StackedDrgWinning2KiBV1,
		abi.RegisteredPoStProof_StackedDrgWinning8MiBV1,
		abi.RegisteredPoStProof_StackedDrgWinning512MiBV1,
		abi.RegisteredPoStProof_StackedDrgWinning32GiBV1,
		abi.RegisteredPoStProof_StackedDrgWinning64GiBV1,
		abi.RegisteredPoStProof_StackedDrgWindow2KiBV1,
		abi.RegisteredPoStProof_StackedDrgWindow8MiBV1,
		abi.RegisteredPoStProof_StackedDrgWindow512MiBV1,
		abi.RegisteredPoStProof_StackedDrgWindow32GiBV1,
		abi.RegisteredPoStProof_StackedDrgWindow64GiBV1,
	}

	for _, pt := range postTypes {
		v, err := GetPoStVersion(pt)
		t.AssertNoError(err)
		t.AssertTrue(len(v) > 0)
	}
}

func WorkflowGenerateWinningPoStSectorChallengeEdgeCase(t TestHelper) {
	for i := 0; i < 10000; i++ {
		var randomnessFr32 [32]byte
		_, err := io.ReadFull(rand.Reader, randomnessFr32[0:31]) // last byte of the 32 is always NUL
		t.RequireNoError(err)

		minerID := abi.ActorID(randUInt64())
		eligibleSectorsLen := uint64(1)

		indices2, err := GenerateWinningPoStSectorChallenge(abi.RegisteredPoStProof_StackedDrgWinning2KiBV1, minerID, randomnessFr32[:], eligibleSectorsLen)
		t.RequireNoError(err)
		t.AssertEqual(1, len(indices2))
		t.AssertEqual(0, int(indices2[0]))
	}
}

func WorkflowGenerateWinningPoStSectorChallenge(t TestHelper) {
	for i := 0; i < 10000; i++ {
		var randomnessFr32 [32]byte
		_, err := io.ReadFull(rand.Reader, randomnessFr32[0:31]) // last byte of the 32 is always NUL
		t.RequireNoError(err)

		minerID := abi.ActorID(randUInt64())
		eligibleSectorsLen := randUInt64()

		if eligibleSectorsLen == 0 {
			continue // no fun
		}

		indices, err := GenerateWinningPoStSectorChallenge(abi.RegisteredPoStProof_StackedDrgWinning2KiBV1, minerID, randomnessFr32[:], eligibleSectorsLen)
		t.AssertNoError(err)

		max := uint64(0)
		for idx := range indices {
			if indices[idx] > max {
				max = indices[idx]
			}
		}

		t.AssertTrue(max < eligibleSectorsLen, "out of range value - max: ", max, "eligibleSectorsLen: ", eligibleSectorsLen)
		t.AssertTrue(uint64(len(indices)) <= eligibleSectorsLen, "should never generate more indices than number of eligible sectors")
	}
}

func randUInt64() uint64 {
	buf := make([]byte, 8)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}

	return binary.LittleEndian.Uint64(buf)
}

func requireTempFile(t TestHelper, fileContentsReader io.Reader, size uint64) *os.File {
	file, err := ioutil.TempFile("", "")
	t.RequireNoError(err)

	written, err := io.Copy(file, fileContentsReader)
	t.RequireNoError(err)
	// check that we wrote everything
	t.RequireEqual(int(size), int(written))

	t.RequireNoError(file.Sync())

	// seek to the beginning
	_, err = file.Seek(0, 0)
	t.RequireNoError(err)

	return file
}

func requireTempDirPath(t TestHelper, prefix string) string {
	dir, err := ioutil.TempDir("", prefix)
	t.RequireNoError(err)

	return dir
}

func visit(paths *[]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			panic(err)
		}
		*paths = append(*paths, path)
		return nil
	}
}

type TestHelper interface {
	AssertEqual(expected, actual interface{}, msgAndArgs ...interface{}) bool
	AssertNoError(err error, msgAndArgs ...interface{}) bool
	AssertTrue(value bool, msgAndArgs ...interface{}) bool
	RequireEqual(expected interface{}, actual interface{}, msgAndArgs ...interface{})
	RequireNoError(err error, msgAndArgs ...interface{})
	RequireTrue(value bool, msgAndArgs ...interface{})
}
