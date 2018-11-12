package testing

import (
	"context"
	"testing"

	msmux "gx/ipfs/QmRYdszNNq7ykPqavVNKMVyyjX59AcTisHqzussDfhwHkK/go-smux-multistream"
	pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
	pstoremem "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore/pstoremem"
	tcp "gx/ipfs/QmTkKN1x5Jvhc5Np55gJzD3PQ6GL74aKm9145t9WbvJyrB/go-tcp-transport"
	csms "gx/ipfs/QmVCtDMyervzvQTi3yZrfTLXtXSuUYQaowCG9hJdHu6gAM/go-conn-security-multistream"
	secio "gx/ipfs/QmXVoVfwEXZdG9zNNHE9in55SepeSJM5GSETEyWNSNMygc/go-libp2p-secio"
	inet "gx/ipfs/QmXuRkCR7BNQa9uqfpTiFWsTQLzmTWYg91Ja1w95gnqb6u/go-libp2p-net"
	tu "gx/ipfs/Qma6ESRQTf1ZLPgzpCwDTqQJefPnU6uLvMjP18vK8EWp8L/go-testutil"
	yamux "gx/ipfs/Qmdps3CYh5htGQSrPvzg5PHouVexLmtpbuLCqc4vuej8PC/go-smux-yamux"
	tptu "gx/ipfs/QmeUjhpfGkrMtE6s7JjU2xLhfzprSDh16QUxGVr3wTrKSx/go-libp2p-transport-upgrader"
	metrics "gx/ipfs/QmeaTjsfPf6vQ3WU2BUdjakgvKUHpuv3Fjxvb75N5iksMx/go-libp2p-metrics"

	swarm "gx/ipfs/QmVHhT8NxtApPTndiZPe4JNGNUxGWtJe3ebyxtRz4HnbEp/go-libp2p-swarm"
)

type config struct {
	disableReuseport bool
	dialOnly         bool
}

// Option is an option that can be passed when constructing a test swarm.
type Option func(*testing.T, *config)

// OptDisableReuseport disables reuseport in this test swarm.
var OptDisableReuseport Option = func(_ *testing.T, c *config) {
	c.disableReuseport = true
}

// OptDialOnly prevents the test swarm from listening.
var OptDialOnly Option = func(_ *testing.T, c *config) {
	c.dialOnly = true
}

// GenUpgrader creates a new connection upgrader for use with this swarm.
func GenUpgrader(n *swarm.Swarm) *tptu.Upgrader {
	id := n.LocalPeer()
	pk := n.Peerstore().PrivKey(id)
	secMuxer := new(csms.SSMuxer)
	secMuxer.AddTransport(secio.ID, &secio.Transport{
		LocalID:    id,
		PrivateKey: pk,
	})

	stMuxer := msmux.NewBlankTransport()
	stMuxer.AddTransport("/yamux/1.0.0", yamux.DefaultTransport)

	return &tptu.Upgrader{
		Secure:  secMuxer,
		Muxer:   stMuxer,
		Filters: n.Filters,
	}

}

// GenSwarm generates a new test swarm.
func GenSwarm(t *testing.T, ctx context.Context, opts ...Option) *swarm.Swarm {
	var cfg config
	for _, o := range opts {
		o(t, &cfg)
	}

	p := tu.RandPeerNetParamsOrFatal(t)

	ps := pstoremem.NewPeerstore()
	ps.AddPubKey(p.ID, p.PubKey)
	ps.AddPrivKey(p.ID, p.PrivKey)
	s := swarm.NewSwarm(ctx, p.ID, ps, metrics.NewBandwidthCounter())

	tcpTransport := tcp.NewTCPTransport(GenUpgrader(s))
	tcpTransport.DisableReuseport = cfg.disableReuseport

	if err := s.AddTransport(tcpTransport); err != nil {
		t.Fatal(err)
	}

	if !cfg.dialOnly {
		if err := s.Listen(p.Addr); err != nil {
			t.Fatal(err)
		}

		s.Peerstore().AddAddrs(p.ID, s.ListenAddresses(), pstore.PermanentAddrTTL)
	}

	return s
}

// DivulgeAddresses adds swarm a's addresses to swarm b's peerstore.
func DivulgeAddresses(a, b inet.Network) {
	id := a.LocalPeer()
	addrs := a.Peerstore().Addrs(id)
	b.Peerstore().AddAddrs(id, addrs, pstore.PermanentAddrTTL)
}
