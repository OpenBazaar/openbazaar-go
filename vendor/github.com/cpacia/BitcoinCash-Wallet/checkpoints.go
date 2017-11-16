package bitcoincash

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"time"
)

type Checkpoint struct {
	Height uint32
	Header wire.BlockHeader
	Check2 *chainhash.Hash
}

var mainnetCheckpoints []Checkpoint
var testnet3Checkpoints []Checkpoint
var regtestCheckpoint Checkpoint

func init() {
	// Mainnet
	mainnetPrev, _ := chainhash.NewHashFromStr("0000000000000000011ebf65b60d0a3de80b8175be709d653b4c1a1beeb6ab9c")
	mainnetMerk, _ := chainhash.NewHashFromStr("8ebf2179d8b1ba0aaf5f15357f963b56f53a8c6207e0156b4b6def119be61bee")
	check2, _ := chainhash.NewHashFromStr("0000000000000000012caacbf01c055790e5a61dda3f4e807552180aa50d8d54")
	mainnetCheckpoints = append(mainnetCheckpoints, Checkpoint{
		Height: 504032,
		Header: wire.BlockHeader{
			Version:    536870912,
			PrevBlock:  *mainnetPrev,
			MerkleRoot: *mainnetMerk,
			Timestamp:  time.Unix(1510606995, 0),
			Bits:       403026987,
			Nonce:      273755974,
		},
		Check2: check2,
	})
	if mainnetCheckpoints[0].Header.BlockHash().String() != "00000000000000000343e9875012f2062554c8752929892c82a0c0743ac7dcfd" {
		panic("Invalid checkpoint")
	}

	// Testnet3
	testnet3Prev, _ := chainhash.NewHashFromStr("00000000824633a21bc41dccbd7a6d159a4deebaece6f6dcf2093301aea040a5")
	testnet3Merk, _ := chainhash.NewHashFromStr("d69264d97d77da1b9bf0ae031512a89e0607e8200be29a74163a39b8558f5714")
	testnetCheck2, _ := chainhash.NewHashFromStr("0000000000005c8804d5e36a166646a9ca1d49250423a851b13d1bbf835e47fe")
	testnet3Checkpoints = append(testnet3Checkpoints, Checkpoint{
		Height: 1189213,
		Header: wire.BlockHeader{
			Version:    536870912,
			PrevBlock:  *testnet3Prev,
			MerkleRoot: *testnet3Merk,
			Timestamp:  time.Unix(1510739152, 0),
			Bits:       486604799,
			Nonce:      2325788686,
		},
		Check2: testnetCheck2,
	})
	if testnet3Checkpoints[0].Header.BlockHash().String() != "000000001f734385476b82be8eb10512c9fb5bd1534cf3ceb4af2d47a7b20ff7" {
		panic("Invalid checkpoint")
	}

	// Regtest
	regtestCheckpoint = Checkpoint{0, chaincfg.RegressionNetParams.GenesisBlock.Header, nil}
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
