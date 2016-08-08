package routedhost

import (
	"fmt"
	"time"

	logging "gx/ipfs/QmNQynaz7qfriSUJkiEZUrm2Wen1u3Kj9goZzWtrPyu7XR/go-log"
	lgbl "gx/ipfs/QmNaS34WZRjs4U1kDfRR2aooYSsBSusFPMEg3dAKk7VZid/go-libp2p-loggables"
	pstore "gx/ipfs/QmQdnfvZQuhdT93LNc5bos52wAmdr3G2p6G8teLJMEN32P/go-libp2p-peerstore"
	peer "gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	host "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/host"
	metrics "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/metrics"
	inet "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/net"
	protocol "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/protocol"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	msmux "gx/ipfs/Qmf91yhgRLo2dhhbc5zZ7TxjMaR1oxaWaoc9zRZdi1kU4a/go-multistream"
)

var log = logging.Logger("github.com/ipfs/go-libp2p/p2p/host/routed")

// AddressTTL is the expiry time for our addresses.
// We expire them quickly.
const AddressTTL = time.Second * 10

// RoutedHost is a p2p Host that includes a routing system.
// This allows the Host to find the addresses for peers when
// it does not have them.
type RoutedHost struct {
	host  host.Host // embedded other host.
	route Routing
}

type Routing interface {
	FindPeer(context.Context, peer.ID) (pstore.PeerInfo, error)
}

func Wrap(h host.Host, r Routing) *RoutedHost {
	return &RoutedHost{h, r}
}

// Connect ensures there is a connection between this host and the peer with
// given peer.ID. See (host.Host).Connect for more information.
//
// RoutedHost's Connect differs in that if the host has no addresses for a
// given peer, it will use its routing system to try to find some.
func (rh *RoutedHost) Connect(ctx context.Context, pi pstore.PeerInfo) error {
	// first, check if we're already connected.
	if len(rh.Network().ConnsToPeer(pi.ID)) > 0 {
		return nil
	}

	// if we were given some addresses, keep + use them.
	if len(pi.Addrs) > 0 {
		rh.Peerstore().AddAddrs(pi.ID, pi.Addrs, pstore.TempAddrTTL)
	}

	// Check if we have some addresses in our recent memory.
	addrs := rh.Peerstore().Addrs(pi.ID)
	if len(addrs) < 1 {

		// no addrs? find some with the routing system.
		pi2, err := rh.route.FindPeer(ctx, pi.ID)
		if err != nil {
			return err // couldnt find any :(
		}
		if pi2.ID != pi.ID {
			err = fmt.Errorf("routing failure: provided addrs for different peer")
			logRoutingErrDifferentPeers(ctx, pi.ID, pi2.ID, err)
			return err
		}
		addrs = pi2.Addrs
	}

	// if we're here, we got some addrs. let's use our wrapped host to connect.
	pi.Addrs = addrs
	return rh.host.Connect(ctx, pi)
}

func logRoutingErrDifferentPeers(ctx context.Context, wanted, got peer.ID, err error) {
	lm := make(lgbl.DeferredMap)
	lm["error"] = err
	lm["wantedPeer"] = func() interface{} { return wanted.Pretty() }
	lm["gotPeer"] = func() interface{} { return got.Pretty() }
	log.Event(ctx, "routingError", lm)
}

func (rh *RoutedHost) ID() peer.ID {
	return rh.host.ID()
}

func (rh *RoutedHost) Peerstore() pstore.Peerstore {
	return rh.host.Peerstore()
}

func (rh *RoutedHost) Addrs() []ma.Multiaddr {
	return rh.host.Addrs()
}

func (rh *RoutedHost) Network() inet.Network {
	return rh.host.Network()
}

func (rh *RoutedHost) Mux() *msmux.MultistreamMuxer {
	return rh.host.Mux()
}

func (rh *RoutedHost) SetStreamHandler(pid protocol.ID, handler inet.StreamHandler) {
	rh.host.SetStreamHandler(pid, handler)
}

func (rh *RoutedHost) RemoveStreamHandler(pid protocol.ID) {
	rh.host.RemoveStreamHandler(pid)
}

func (rh *RoutedHost) NewStream(ctx context.Context, pid protocol.ID, p peer.ID) (inet.Stream, error) {
	return rh.host.NewStream(ctx, pid, p)
}
func (rh *RoutedHost) Close() error {
	// no need to close IpfsRouting. we dont own it.
	return rh.host.Close()
}

func (rh *RoutedHost) GetBandwidthReporter() metrics.Reporter {
	return rh.host.GetBandwidthReporter()
}
