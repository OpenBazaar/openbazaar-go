package test

import (
	// "github.com/ipfs/go-ipfs/thirdparty/testutil"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg"
	"gx/ipfs/QmWUswjn261LSyVxWAEpMVtPdy8zmKBJJfBpG3Qdpa8ZsE/go-libp2p-peer"
)

// NewNode creates a new *core.OpenBazaarNode prepared for testing
func NewNode() (*core.OpenBazaarNode, error) {
	// Create test repo
	repository, err := NewRepository()
	if err != nil {
		return nil, err
	}

	repository.Reset()
	if err != nil {
		return nil, err
	}

	// Create test ipfs node
	ipfsNode, err := ipfs.NewMockNode()
	if err != nil {
		return nil, err
	}

	// Create test context
	ctx, err := ipfs.MockCmdsCtx()
	if err != nil {
		return nil, err
	}

	// Create test wallet
	mnemonic, err := repository.DB.Config().GetMnemonic()
	if err != nil {
		return nil, err
	}

	walletCfg, err := repo.GetWalletConfig(repository.ConfigFile())
	if err != nil {
		return nil, err
	}

	wallet, err := spvwallet.NewSPVWallet(
		mnemonic,
		&chaincfg.TestNet3Params, 50000, 8000, 16000, 24000, walletCfg.FeeAPI,
		repository.Path,
		repository.DB,
		"OpenBazaar-Test",
		walletCfg.TrustedPeer,
		nil,
		NewLogger(),
	)
	if err != nil {
		return nil, err
	}

	// Put it all together in an OpenBazaarNode
	node := &core.OpenBazaarNode{
		Context:    ctx,
		RepoPath:   GetRepoPath(),
		IpfsNode:   ipfsNode,
		Datastore:  repository.DB,
		Wallet:     wallet,
		BanManager: net.NewBanManager([]peer.ID{}),
	}

	node.Service = service.New(node, ctx, repository.DB)

	return node, nil
}
