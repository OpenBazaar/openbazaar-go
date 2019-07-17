package ipfs

import (
	"context"
	"fmt"

	libp2p "gx/ipfs/QmRxk6AUaGaKCfzS1xSNRojiAPd7h2ih8GuCdjJBF3Y6GK/go-libp2p"
	dht "gx/ipfs/QmSY3nkMNLzh9GdbFKK5tT7YMfLpf52iUZ8ZRkr29MJaa5/go-libp2p-kad-dht"
	dhtopts "gx/ipfs/QmSY3nkMNLzh9GdbFKK5tT7YMfLpf52iUZ8ZRkr29MJaa5/go-libp2p-kad-dht/opts"
	ma "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	ds "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	p2phost "gx/ipfs/QmYrWiWM4qtrnCeT3R14jY3ZZyirDNJgwK57q4qFYePgbd/go-libp2p-host"
	routing "gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing"
	pstore "gx/ipfs/QmaCTz9RkrU13bm9kMB54f7atgqM4qkjDZpRwRoJiWXEqs/go-libp2p-peerstore"
	record "gx/ipfs/QmbeHtaBy9nZsW4cHRcvgVY4CnDhXudE2Dr6qDxS7yg9rX/go-libp2p-record"
	manet "gx/ipfs/Qmc85NSvmSG4Frn9Vb2cBc1rMyULH6D3TNVEfCzSKoUpip/go-multiaddr-net"
	bitswap "gx/ipfs/QmcSPuzpSbVLU6UHU4e5PwZpm4fHbCn5SbNR5ZNL6Mj63G/go-bitswap/network"

	ipfscore "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo"
)

var routerCacheURI string

// UpdateIPFSGlobalProtocolVars is a hack to manage custom protocol strings
// which do not yet have an API to manage their configuration
func UpdateIPFSGlobalProtocolVars(testnetEnable bool) {
	if testnetEnable {
		bitswap.ProtocolBitswap = IPFSProtocolBitswapTestnetOneDotOne
		bitswap.ProtocolBitswapOne = IPFSProtocolBitswapTestnetOne
		bitswap.ProtocolBitswapNoVers = IPFSProtocolBitswapTestnetNoVers
	} else {
		bitswap.ProtocolBitswap = IPFSProtocolBitswapMainnetOneDotOne
		bitswap.ProtocolBitswapOne = IPFSProtocolBitswapMainnetOne
		bitswap.ProtocolBitswapNoVers = IPFSProtocolBitswapMainnetNoVers
	}
}

func useStaticRelayCircuits(addrs []ma.Multiaddr) []ma.Multiaddr {
	staticCircuits := []string{
		"/ip4/138.68.10.227/tcp/4001/ipfs/QmWUdwXW3bTXS19MtMjmfpnRYgssmbJCwnq8Lf9vjZwDii/p2p-circuit/",
		"/ip4/157.230.59.219/tcp/4001/ipfs/QmcXwJePGLsP1x7gTXLE51BmE7peUKe2eQuR5LGbmasekt/p2p-circuit/",
		"/ip4/206.189.224.90/tcp/4001/ipfs/Qmb8i7uy6rk47hNorNLMVRMer4Nv9YWRhzZrWVqnvk5mSk/p2p-circuit/",
	}
	raddrs := make([]ma.Multiaddr, 0, len(staticCircuits)+len(addrs))
	for _, addr := range addrs {
		if manet.IsPrivateAddr(addr) {
			raddrs = append(raddrs, addr)
		}
	}
	for _, relayCircuit := range staticCircuits {
		relayMultiaddr, err := ma.NewMultiaddr(relayCircuit)
		if err != nil {
			panic("static relay circuit is invalid, unable to construct mobile p2p host")
		}
		raddrs = append(raddrs, relayMultiaddr)
	}

	return raddrs
}

func constructMobileHost(ctx context.Context, id peer.ID, ps pstore.Peerstore, options ...libp2p.Option) (p2phost.Host, error) {
	pkey := ps.PrivKey(id)
	if pkey == nil {
		return nil, fmt.Errorf("missing private key for node ID: %s", id.Pretty())
	}

	options = append([]libp2p.Option{
		libp2p.Identity(pkey),
		libp2p.Peerstore(ps),
		libp2p.AddrsFactory(useStaticRelayCircuits),
	}, options...)
	return libp2p.New(ctx, options...)
}

// PrepareIPFSConfig builds the configuration options for the internal
// IPFS node.
func PrepareIPFSConfig(r repo.Repo, routerAPIEndpoint string, testEnable, regtestEnable bool) *ipfscore.BuildCfg {
	routerCacheURI = routerAPIEndpoint
	ncfg := &ipfscore.BuildCfg{
		Repo:   r,
		Online: true,
		ExtraOpts: map[string]bool{
			"mplex":  true,
			"ipnsps": true,
		},
	}

	// regtest and test are never enabled together
	ncfg.Routing = constructRouting
	if regtestEnable {
		ncfg.Routing = constructRegtestRouting
	} else if testEnable {
		ncfg.Routing = constructTestnetRouting
	}
	ncfg.Host = constructMobileHost
	return ncfg
}

func constructRouting(ctx context.Context, host p2phost.Host, dstore ds.Batching, validator record.Validator) (routing.IpfsRouting, error) {
	dhtRouting, err := dht.New(
		ctx, host,
		dhtopts.Datastore(dstore),
		dhtopts.Validator(validator),
		dhtopts.Protocols(
			IPFSProtocolKademliaMainnetOne,
			IPFSProtocolDHTMainnetLegacy,
		),
	)
	if err != nil {
		return nil, err
	}
	apiRouter := NewAPIRouter(routerCacheURI, dhtRouting.Validator)
	cachingRouter := NewCachingRouter(dhtRouting, &apiRouter)
	return cachingRouter, nil
}

func constructRegtestRouting(ctx context.Context, host p2phost.Host, dstore ds.Batching, validator record.Validator) (routing.IpfsRouting, error) {
	return dht.New(
		ctx, host,
		dhtopts.Datastore(dstore),
		dhtopts.Validator(validator),
		dhtopts.Protocols(
			IPFSProtocolKademliaMainnetOne,
			IPFSProtocolDHTMainnetLegacy,
		),
	)
}

func constructTestnetRouting(ctx context.Context, host p2phost.Host, dstore ds.Batching, validator record.Validator) (routing.IpfsRouting, error) {
	var (
		dhtRouting, err = dht.New(
			ctx, host,
			dhtopts.Datastore(dstore),
			dhtopts.Validator(validator),
			dhtopts.Protocols(
				IPFSProtocolKademliaTestnetOne,
				IPFSProtocolAppTestnetOne,
			),
		)
	)
	if err != nil {
		return nil, err
	}
	apiRouter := NewAPIRouter(routerCacheURI, dhtRouting.Validator)
	cachingRouter := NewCachingRouter(dhtRouting, &apiRouter)
	return cachingRouter, nil
}
