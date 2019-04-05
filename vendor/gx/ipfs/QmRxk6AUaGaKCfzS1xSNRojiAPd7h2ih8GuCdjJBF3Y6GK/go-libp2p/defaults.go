package libp2p

// This file contains all the default configuration options.

import (
	"crypto/rand"

	secio "gx/ipfs/QmSVaJe1aRjc78cZARTtf4pqvXERYwihyYhZWoVWceHnsK/go-libp2p-secio"
	tcp "gx/ipfs/QmTGiDkw4eeKq31wwpQRk5GwWiReaxrcTQLuCCLWgfKo5M/go-tcp-transport"
	crypto "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	multiaddr "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	pstoremem "gx/ipfs/QmaCTz9RkrU13bm9kMB54f7atgqM4qkjDZpRwRoJiWXEqs/go-libp2p-peerstore/pstoremem"
	mplex "gx/ipfs/QmaJvNdDccVkTELQLCGXWrLxgaQ14aMdhzZx1EiHPXKbDc/go-smux-multiplex"
	ws "gx/ipfs/QmaSWc4ox6SZQF6DHZvDuM9sP1syNajkKuPXmKR1t5BAz5/go-ws-transport"
	yamux "gx/ipfs/QmcVFwGS6sjfxVico2bd1gQGRu5A2ymwZunVmMdeV5zEYb/go-smux-yamux"
)

// DefaultSecurity is the default security option.
//
// Useful when you want to extend, but not replace, the supported transport
// security protocols.
var DefaultSecurity = Security(secio.ID, secio.New)

// DefaultMuxers configures libp2p to use the stream connection multiplexers.
//
// Use this option when you want to *extend* the set of multiplexers used by
// libp2p instead of replacing them.
var DefaultMuxers = ChainOptions(
	Muxer("/yamux/1.0.0", yamux.DefaultTransport),
	Muxer("/mplex/6.7.0", mplex.DefaultTransport),
)

// DefaultTransports are the default libp2p transports.
//
// Use this option when you want to *extend* the set of transports used by
// libp2p instead of replacing them.
var DefaultTransports = ChainOptions(
	Transport(tcp.NewTCPTransport),
	Transport(ws.New),
)

// DefaultPeerstore configures libp2p to use the default peerstore.
var DefaultPeerstore Option = func(cfg *Config) error {
	return cfg.Apply(Peerstore(pstoremem.NewPeerstore()))
}

// RandomIdentity generates a random identity (default behaviour)
var RandomIdentity = func(cfg *Config) error {
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	if err != nil {
		return err
	}
	return cfg.Apply(Identity(priv))
}

// DefaultListenAddrs configures libp2p to use default listen address
var DefaultListenAddrs = func(cfg *Config) error {
	defaultIP4ListenAddr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
	if err != nil {
		return err
	}

	defaultIP6ListenAddr, err := multiaddr.NewMultiaddr("/ip6/::/tcp/0")
	if err != nil {
		return err
	}
	return cfg.Apply(ListenAddrs(
		defaultIP4ListenAddr,
		defaultIP6ListenAddr,
	))
}

// DefaultEnableRelay enables relay dialing and listening by default
var DefaultEnableRelay = func(cfg *Config) error {
	return cfg.Apply(EnableRelay())
}

// Complete list of default options and when to fallback on them.
//
// Please *DON'T* specify default options any other way. Putting this all here
// makes tracking defaults *much* easier.
var defaults = []struct {
	fallback func(cfg *Config) bool
	opt      Option
}{
	{
		fallback: func(cfg *Config) bool { return cfg.Transports == nil && cfg.ListenAddrs == nil },
		opt:      DefaultListenAddrs,
	},
	{
		fallback: func(cfg *Config) bool { return cfg.Transports == nil },
		opt:      DefaultTransports,
	},
	{
		fallback: func(cfg *Config) bool { return cfg.Muxers == nil },
		opt:      DefaultMuxers,
	},
	{
		fallback: func(cfg *Config) bool { return !cfg.Insecure && cfg.SecurityTransports == nil },
		opt:      DefaultSecurity,
	},
	{
		fallback: func(cfg *Config) bool { return cfg.PeerKey == nil },
		opt:      RandomIdentity,
	},
	{
		fallback: func(cfg *Config) bool { return cfg.Peerstore == nil },
		opt:      DefaultPeerstore,
	},
	{
		fallback: func(cfg *Config) bool { return !cfg.RelayCustom },
		opt:      DefaultEnableRelay,
	},
}

// Defaults configures libp2p to use the default options. Can be combined with
// other options to *extend* the default options.
var Defaults Option = func(cfg *Config) error {
	for _, def := range defaults {
		if err := cfg.Apply(def.opt); err != nil {
			return err
		}
	}
	return nil
}

// FallbackDefaults applies default options to the libp2p node if and only if no
// other relevent options have been applied. will be appended to the options
// passed into New.
var FallbackDefaults Option = func(cfg *Config) error {
	for _, def := range defaults {
		if !def.fallback(cfg) {
			continue
		}
		if err := cfg.Apply(def.opt); err != nil {
			return err
		}
	}
	return nil
}
