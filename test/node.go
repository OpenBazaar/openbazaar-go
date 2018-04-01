package test

import (
	"sync"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	repodb "github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/tyler-smith/go-bip39"
	"gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

// NewNode creates a new *core.OpenBazaarNode prepared for testing
func NewNode(context schema.SchemaContext) (*core.OpenBazaarNode, error) {
	appSchema := schema.MustNewCustomSchemaManager(context)
	if err := appSchema.BuildSchemaDirectories(); err != nil {
		return nil, err
	}
	// TODO: defer appSchema.DestroySchemaDirectories() doesn't work here

	if err := appSchema.InitializeDatabase(); err != nil {
		return nil, err
	}
	if err := appSchema.InitializeIPFSRepo(); err != nil {
		return nil, err
	}
	db, err := appSchema.OpenDatabase()
	if err != nil {
		return nil, err
	}
	datastore := repodb.NewSQLiteDatastore(db, new(sync.Mutex))

	// Create test ipfs node
	ipfsNode, err := ipfs.NewMockNode()
	if err != nil {
		return nil, err
	}

	seed := bip39.NewSeed(context.SchemaPassword, "Secret Passphrase")
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
	spvwalletConfig := &spvwallet.Config{
		Mnemonic:    appSchema.Mnemonic(),
		Params:      &chaincfg.TestNet3Params,
		MaxFee:      50000,
		LowFee:      8000,
		MediumFee:   16000,
		HighFee:     24000,
		RepoPath:    appSchema.DataPath(),
		DB:          datastore,
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
		RepoPath:   appSchema.DataPath(),
		IpfsNode:   ipfsNode,
		Datastore:  datastore,
		Wallet:     wallet,
		BanManager: net.NewBanManager([]peer.ID{}),
	}

	node.Service = service.New(node, ctx, datastore)

	return node, nil
}
