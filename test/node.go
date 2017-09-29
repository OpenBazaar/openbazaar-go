package test

import (
	// "github.com/ipfs/go-ipfs/thirdparty/testutil"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/tyler-smith/go-bip39"
	"gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
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

	seed := bip39.NewSeed(GetPassword(), "Secret Passphrase")
	privKey, err := ipfs.IdentityKeyFromSeed(seed, 256)
	if err != nil {
		return nil, err
	}

	sk, err := crypto.UnmarshalPrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	id, err := peer.IDFromPublicKey(sk.GetPublic())
	if err != nil {
		return nil, err
	}

	ipfsNode.Identity = id

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
	spvwalletConfig := &spvwallet.Config{
		Mnemonic:    mnemonic,
		Params:      &chaincfg.TestNet3Params,
		MaxFee:      50000,
		LowFee:      8000,
		MediumFee:   16000,
		HighFee:     24000,
		RepoPath:    repository.Path,
		DB:          repository.DB,
		UserAgent:   "OpenBazaar",
		TrustedPeer: nil,
		Proxy:       nil,
		Logger:      NewLogger(),
	}

	wallet, err := spvwallet.NewSPVWallet(spvwalletConfig)
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
