package mobile_test

import (
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/mobile"
	"github.com/OpenBazaar/openbazaar-go/schema"
	bitswap "gx/ipfs/QmcSPuzpSbVLU6UHU4e5PwZpm4fHbCn5SbNR5ZNL6Mj63G/go-bitswap/network"
	"testing"
)

func TestBitswapProtocolSetCorrectly(t *testing.T) {
	var s, err = schema.NewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.DestroySchemaDirectories()

	if err := s.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}

	config := &mobile.NodeConfig{
		RepoPath:             s.DataPath(),
		DisableWallet:        true,
		DisableExchangerates: true,
		Testnet:              true,
	}
	_, err = mobile.NewNodeWithConfig(config, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if bitswap.ProtocolBitswap != ipfs.IPFSProtocolBitswapTestnetOneDotOne {
		t.Errorf("expected bitswap protocol latest to be set to '%s' when using testnet, but was not", ipfs.IPFSProtocolBitswapTestnetOneDotOne)
	}
	if bitswap.ProtocolBitswapOne != ipfs.IPFSProtocolBitswapTestnetOne {
		t.Errorf("expected bitswap protocol v1 to be set to '%s' when using testnet, but was not", ipfs.IPFSProtocolBitswapTestnetOne)
	}
	if bitswap.ProtocolBitswapNoVers != ipfs.IPFSProtocolBitswapTestnetNoVers {
		t.Errorf("expected bitswap protocol nover to be set to '%s' when using testnet, but was not", ipfs.IPFSProtocolBitswapTestnetNoVers)
	}
}
