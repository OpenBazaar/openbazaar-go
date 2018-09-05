package test

import (
	// "github.com/ipfs/go-ipfs/thirdparty/testutil"
	"github.com/OpenBazaar/multiwallet"
	"github.com/OpenBazaar/multiwallet/config"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	"github.com/OpenBazaar/spvwallet"
	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/ipfs/go-ipfs/core/mock"
	"github.com/tyler-smith/go-bip39"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	inet "net"
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
	ipfsNode, err := coremock.NewMockNode()
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

	// Create test wallet
	mnemonic, err := repository.DB.Config().GetMnemonic()
	if err != nil {
		return nil, err
	}
	mPrivKey, err := hdkeychain.NewMaster(seed, &chaincfg.RegressionNetParams)
	if err != nil {
		return nil, err
	}
	tp, err := inet.ResolveTCPAddr("tcp4", "127.0.0.1:8333")
	if err != nil {
		return nil, err
	}
	spvwalletConfig := &spvwallet.Config{
		Mnemonic:    mnemonic,
		Params:      &chaincfg.RegressionNetParams,
		MaxFee:      50000,
		LowFee:      8000,
		MediumFee:   16000,
		HighFee:     24000,
		RepoPath:    repository.Path,
		DB:          repository.DB,
		UserAgent:   "OpenBazaar",
		TrustedPeer: tp,
		Proxy:       nil,
		Logger:      NewLogger(),
	}

	coins := make(map[wi.CoinType]bool)
	coins[wi.Bitcoin] = true
	coins[wi.BitcoinCash] = true
	coins[wi.Zcash] = true
	coins[wi.Litecoin] = true

	walletConf := config.NewDefaultConfig(coins, &chaincfg.RegressionNetParams)
	walletConf.Mnemonic = mnemonic
	mw, err := multiwallet.NewMultiWallet(walletConf)
	if err != nil {
		return nil, err
	}

	wallet, err := spvwallet.NewSPVWallet(spvwalletConfig)
	if err != nil {
		return nil, err
	}

	// Put it all together in an OpenBazaarNode
	node := &core.OpenBazaarNode{
		RepoPath:         GetRepoPath(),
		IpfsNode:         ipfsNode,
		Datastore:        repository.DB,
		Wallet:           wallet,
		Multiwallet:      mw,
		BanManager:       net.NewBanManager([]peer.ID{}),
		MasterPrivateKey: mPrivKey,
	}

	node.Service = service.New(node, repository.DB)

	return node, nil
}
