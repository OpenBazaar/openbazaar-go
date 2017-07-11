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
	mainnetPrev, _ := chainhash.NewHashFromStr("000000000000000000b3ff31d54e9e83515ee18360c7dc59e30697d083c745ff")
	mainnetMerk, _ := chainhash.NewHashFromStr("33d4a902daa28d09f9f6a319f538153e4b747938e20e113a2935c8dc0b971584")
	mainnetCheckpoints = append(mainnetCheckpoints, Checkpoint{
		Height: 443520,
		Header: wire.BlockHeader{
			Version:    536870912,
			PrevBlock:  *mainnetPrev,
			MerkleRoot: *mainnetMerk,
			Timestamp:  time.Unix(1481765313, 0),
			Bits:       402885509,
			Nonce:      251583942,
		},
	})

	testnet3Prev, _ := chainhash.NewHashFromStr("00000000000016abe4e7c10ddb658bb089b2ef3b1de3f3329097cf679eedf2b5")
	testnet3Merk, _ := chainhash.NewHashFromStr("ba732d7a0e4b0b46351b1b476e1628ff03f399ce07f888a257982240b36e2ed2")
	testnet3Checkpoints = append(testnet3Checkpoints, Checkpoint{
		Height: 1114848,
		Header: wire.BlockHeader{
			Version:    536870912,
			PrevBlock:  *testnet3Prev,
			MerkleRoot: *testnet3Merk,
			Timestamp:  time.Unix(1491041521, 0),
			Bits:       438809536,
			Nonce:      2732625067,
		},
	})
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
