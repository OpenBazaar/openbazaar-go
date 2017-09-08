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
