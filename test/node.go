package test

import (
	"context"
	"gx/ipfs/QmRxk6AUaGaKCfzS1xSNRojiAPd7h2ih8GuCdjJBF3Y6GK/go-libp2p"
	"gx/ipfs/QmSY3nkMNLzh9GdbFKK5tT7YMfLpf52iUZ8ZRkr29MJaa5/go-libp2p-kad-dht"
	"gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"

	"github.com/OpenBazaar/multiwallet"
	"github.com/OpenBazaar/multiwallet/config"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/net/service"
	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/ipfs/go-ipfs/core/mock"
	"github.com/tyler-smith/go-bip39"
)

// NewNode creates a new *core.OpenBazaarNode prepared for testing
func NewNode() (*core.OpenBazaarNode, error) {
	// Create test repo
	repository, err := NewRepository()
	if err != nil {
		return nil, err
	}

	err = repository.Reset()
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

	coins := make(map[wi.CoinType]bool)
	coins[wi.Bitcoin] = true
	coins[wi.BitcoinCash] = true
	coins[wi.Zcash] = true
	coins[wi.Litecoin] = true

	walletConf := config.NewDefaultConfig(coins, &chaincfg.RegressionNetParams)
	walletConf.Mnemonic = mnemonic
	walletConf.DisableExchangeRates = true
	mw, err := multiwallet.NewMultiWallet(walletConf)
	if err != nil {
		return nil, err
	}
	host, err := libp2p.New(context.Background())
	if err != nil {
		return nil, err
	}
	routing, err := dht.New(context.Background(), host)
	if err != nil {
		return nil, err
	}
	close(routing.BootstrapChan)

	// Put it all together in an OpenBazaarNode
	node := &core.OpenBazaarNode{
		RepoPath:         GetRepoPath(),
		IpfsNode:         ipfsNode,
		Datastore:        repository.DB,
		Multiwallet:      mw,
		BanManager:       net.NewBanManager([]peer.ID{}),
		MasterPrivateKey: mPrivKey,
		DHT:              routing,
		TestnetEnable:    true,
	}

	node.Service = service.New(node, repository.DB)

	return node, nil
}
