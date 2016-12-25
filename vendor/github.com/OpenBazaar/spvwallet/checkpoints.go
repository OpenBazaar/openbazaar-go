package spvwallet

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"time"
)

const (
	MAINNET_CHECKPOINT_HEIGHT  = 443520
	TESTNET3_CHECKPOINT_HEIGHT = 1058400
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

	testnet3Prev, _ := chainhash.NewHashFromStr("00000000000008471ccf356a18dd48aa12506ef0b6162cb8f98a8d8bb0465902")
	testnet3Merk, _ := chainhash.NewHashFromStr("a2bd975d9ac68eb1a7bc00df593c55a64e81ac0c9b8f535bb06b390d3010816f")
	testnet3Checkpoint = wire.BlockHeader{
		Version:    536870912,
		PrevBlock:  *testnet3Prev,
		MerkleRoot: *testnet3Merk,
		Timestamp:  time.Unix(1481479754, 0),
		Bits:       436861323,
		Nonce:      3058617296,
	}
	regtestCheckpoint = chaincfg.RegressionNetParams.GenesisBlock.Header
}
