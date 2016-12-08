package test

import (
	"testing"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg"
)

// NewNode creates a new *core.OpenBazaarNode prepared for testing
func NewNode(t *testing.T) *core.OpenBazaarNode {
	// Create test repo
	repository, err := NewRepository()
	if err != nil {
		t.Fatal(err)
	}

	// Create test ipfs node
	ipfsNode, err := ipfs.NewMockNode()
	if err != nil {
		t.Fatal(err)
	}

	// Create test context
	ctx, err := ipfs.MockCmdsCtx()
	if err != nil {
		t.Fatal(err)
	}

	// Create test wallet
	mnemonic, err := repository.DB.Config().GetMnemonic()
	if err != nil {
		t.Fatal(err)
	}

	walletCfg, err := repo.GetWalletConfig(repository.ConfigFile())
	if err != nil {
		t.Fatal(err)
	}

	wallet := spvwallet.NewSPVWallet(
		mnemonic,
		&chaincfg.TestNet3Params, 50000, 8000, 16000, 24000, walletCfg.FeeAPI,
		repository.Path,
		repository.DB,
		"OpenBazaar-Test",
		walletCfg.TrustedPeer,
		NewLogger(),
	)

	// Put it all together in an OpenBazaarNode!
	node := &core.OpenBazaarNode{
		Context:   ctx,
		RepoPath:  GetRepoPath(),
		IpfsNode:  ipfsNode,
		Datastore: repository.DB,
		Wallet:    wallet,
	}

	node.Service = service.New(node, ctx, repository.DB)

	return node
}
