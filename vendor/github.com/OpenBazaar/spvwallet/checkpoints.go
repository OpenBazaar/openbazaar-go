package spvwallet

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"time"
)

type Checkpoint struct {
	Height uint32
	Header wire.BlockHeader
}

var mainnetCheckpoints []Checkpoint
var testnet3Checkpoints []Checkpoint
var regtestCheckpoint Checkpoint

func init() {
	// Mainnet
	mainnetPrev, _ := chainhash.NewHashFromStr("000000000000000001389446206ebcd378c32cd00b4920a8a1ba7b540ca7d699")
	mainnetMerk, _ := chainhash.NewHashFromStr("ddc4fede55aeebe6e3bfd3292145b011a4f16ead187ed90d7df0fd4c020b6ab6")
	mainnetCheckpoints = append(mainnetCheckpoints, Checkpoint{
		Height: 473760,
		Header: wire.BlockHeader{
			Version:    536870914,
			PrevBlock:  *mainnetPrev,
			MerkleRoot: *mainnetMerk,
			Timestamp:  time.Unix(1498956437, 0),
			Bits:       402754864,
			Nonce:      134883004,
		},
	})
	if mainnetCheckpoints[0].Header.BlockHash().String() != "000000000000000000802ba879f1b7a638dcea6ff0ceb614d91afc8683ac0502" {
		panic("Invalid checkpoint")
	}

	mainnetPrev1, _ := chainhash.NewHashFromStr("000000000000000000d0ad638ad61e7c4c3113618b8b26b2044347c00c042278")
	mainnetMerk1, _ := chainhash.NewHashFromStr("87b940030e48d97625b923c3ebc0626c2cb1123b78135380306eb6dcfd50703c")
	mainnetCheckpoints = append(mainnetCheckpoints, Checkpoint{
		Height: 483840,
		Header: wire.BlockHeader{
			Version:    536870912,
			PrevBlock:  *mainnetPrev1,
			MerkleRoot: *mainnetMerk1,
			Timestamp:  time.Unix(1504704195, 0),
			Bits:       402731275,
			Nonce:      1775134070,
		},
	})
	if mainnetCheckpoints[1].Header.BlockHash().String() != "0000000000000000008e5d72027ef42ca050a0776b7184c96d0d4b300fa5da9e" {
		panic("Invalid checkpoint")
	}

	mainnetPrev2, _ := chainhash.NewHashFromStr("00000000000000000000943de85f4495f053ff55f27d135edc61c27990c2eec5")
	mainnetMerk2, _ := chainhash.NewHashFromStr("167bf70981d49388d07881b1a448ff9b79cf2a32716e45c535345823d8cdd541")
	mainnetCheckpoints = append(mainnetCheckpoints, Checkpoint{
		Height: 536256,
		Header: wire.BlockHeader{
			Version:    536870912,
			PrevBlock:  *mainnetPrev2,
			MerkleRoot: *mainnetMerk2,
			Timestamp:  time.Unix(1533980459, 0),
			Bits:       388763047,
			Nonce:      1545867530,
		},
	})
	if mainnetCheckpoints[2].Header.BlockHash().String() != "000000000000000000262e508512ce2e6a018e181fb2e5efe048a4e01d21fa7a" {
		panic("Invalid checkpoint")
	}

	// Testnet3
	testnet3Prev, _ := chainhash.NewHashFromStr("0000000000001e8cdb2d98471a5c60bdbddbe644b9ad08e17a97b3a7dce1e332")
	testnet3Merk, _ := chainhash.NewHashFromStr("f675c565b293be2ad808b01b0a763557c8874e4aefe7f2eea0dab91b1f60ec45")
	testnet3Checkpoints = append(testnet3Checkpoints, Checkpoint{
		Height: 1151136,
		Header: wire.BlockHeader{
			Version:    536870912,
			PrevBlock:  *testnet3Prev,
			MerkleRoot: *testnet3Merk,
			Timestamp:  time.Unix(1498950206, 0),
			Bits:       436724869,
			Nonce:      2247874206,
		},
	})
	if testnet3Checkpoints[0].Header.BlockHash().String() != "00000000000002c04de174cf25c993b4dd221eb087c0601a599ff1977e230c99" {
		panic("Invalid checkpoint")
	}
	testnet3Prev1, _ := chainhash.NewHashFromStr("000000000003e8e7755d9b8299b28c71d9f0e18909f25bc9f3eeec3464ece1dd")
	testnet3Merk1, _ := chainhash.NewHashFromStr("7b91fe22059063bcbb1cfac6fd376cf459f4387d1bc1989989252495b06b52be")
	testnet3Checkpoints = append(testnet3Checkpoints, Checkpoint{
		Height: 1276128,
		Header: wire.BlockHeader{
			Version:    536870912,
			PrevBlock:  *testnet3Prev1,
			MerkleRoot: *testnet3Merk1,
			Timestamp:  time.Unix(1517822323, 0),
			Bits:       453210804,
			Nonce:      2456211891,
		},
	})
	if testnet3Checkpoints[1].Header.BlockHash().String() != "0000000000006c7a8a7fae87866c1962460d50bdcaccb53fa59e5456711c4ec8" {
		panic("Invalid checkpoint")
	}
	testnet3Prev2, _ := chainhash.NewHashFromStr("000000000000006d4025181f5b54cca6d730cc26313817c6529ba9ed62cc83b3")
	testnet3Merk2, _ := chainhash.NewHashFromStr("67b8699931cc1f02e7593b0a504f406b0be1bf29cb66b41455ea7b26d4043937")
	testnet3Checkpoints = append(testnet3Checkpoints, Checkpoint{
		Height: 1382976,
		Header: wire.BlockHeader{
			Version:    536870912,
			PrevBlock:  *testnet3Prev2,
			MerkleRoot: *testnet3Merk2,
			Timestamp:  time.Unix(1533934105, 0),
			Bits:       425273147,
			Nonce:      1715685244,
		},
	})
	if testnet3Checkpoints[2].Header.BlockHash().String() != "0000000000000030029452f154927ec30e2efbdd2ac6e7c1b551917de081b16f" {
		panic("Invalid checkpoint")
	}

	// Regtest
	regtestCheckpoint = Checkpoint{0, chaincfg.RegressionNetParams.GenesisBlock.Header}
}

func GetCheckpoint(walletCreationDate time.Time, params *chaincfg.Params) Checkpoint {
	switch params.Name {
	case chaincfg.MainNetParams.Name:
		for i := len(mainnetCheckpoints) - 1; i >= 0; i-- {
			if walletCreationDate.After(mainnetCheckpoints[i].Header.Timestamp) {
				return mainnetCheckpoints[i]
			}
		}
		return mainnetCheckpoints[0]
	case chaincfg.TestNet3Params.Name:
		for i := len(testnet3Checkpoints) - 1; i >= 0; i-- {
			if walletCreationDate.After(testnet3Checkpoints[i].Header.Timestamp) {
				return testnet3Checkpoints[i]
			}
		}
		return testnet3Checkpoints[0]

	default:
		return regtestCheckpoint
	}
}
