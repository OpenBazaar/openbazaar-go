package spvwallet

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"time"
)

const (
	MAINNET_CHECKPOINT_HEIGHT  = 443520
	TESTNET3_CHECKPOINT_HEIGHT = 1114848
	REGTEST_CHECKPOINT_HEIGHT  = 0
)

var mainnetCheckpoint wire.BlockHeader
var testnet3Checkpoint wire.BlockHeader
var regtestCheckpoint wire.BlockHeader

func init() {
	mainnetPrev, _ := chainhash.NewHashFromStr("000000000000000000b3ff31d54e9e83515ee18360c7dc59e30697d083c745ff")
	mainnetMerk, _ := chainhash.NewHashFromStr("33d4a902daa28d09f9f6a319f538153e4b747938e20e113a2935c8dc0b971584")
	mainnetCheckpoint = wire.BlockHeader{
		Version:    536870912,
		PrevBlock:  *mainnetPrev,
		MerkleRoot: *mainnetMerk,
		Timestamp:  time.Unix(1481765313, 0),
		Bits:       402885509,
		Nonce:      251583942,
	}

	testnet3Prev, _ := chainhash.NewHashFromStr("00000000000016abe4e7c10ddb658bb089b2ef3b1de3f3329097cf679eedf2b5")
	testnet3Merk, _ := chainhash.NewHashFromStr("ba732d7a0e4b0b46351b1b476e1628ff03f399ce07f888a257982240b36e2ed2")
	testnet3Checkpoint = wire.BlockHeader{
		Version:    536870912,
		PrevBlock:  *testnet3Prev,
		MerkleRoot: *testnet3Merk,
		Timestamp:  time.Unix(1491041521, 0),
		Bits:       438809536,
		Nonce:      2732625067,
	}
	regtestCheckpoint = chaincfg.RegressionNetParams.GenesisBlock.Header
}
