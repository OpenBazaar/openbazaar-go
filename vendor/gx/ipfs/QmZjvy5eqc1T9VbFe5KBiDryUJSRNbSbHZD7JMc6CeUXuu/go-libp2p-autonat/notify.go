package autonat

import (
	"time"

	ma "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	inet "gx/ipfs/QmY3ArotKMKaL7YGfbQfyDrib6RVraLqZYWXZvVgZktBxp/go-libp2p-net"
)

var _ inet.Notifiee = (*AmbientAutoNAT)(nil)

var AutoNATIdentifyDelay = 5 * time.Second

func (as *AmbientAutoNAT) Listen(net inet.Network, a ma.Multiaddr)      {}
func (as *AmbientAutoNAT) ListenClose(net inet.Network, a ma.Multiaddr) {}
func (as *AmbientAutoNAT) OpenedStream(net inet.Network, s inet.Stream) {}
func (as *AmbientAutoNAT) ClosedStream(net inet.Network, s inet.Stream) {}

func (as *AmbientAutoNAT) Connected(net inet.Network, c inet.Conn) {
	p := c.RemotePeer()

	go func() {
		// add some delay for identify
		time.Sleep(AutoNATIdentifyDelay)

		protos, err := as.host.Peerstore().SupportsProtocols(p, AutoNATProto)
		if err != nil {
			log.Debugf("error retrieving supported protocols for peer %s: %s", p, err)
			return
		}

		if len(protos) > 0 {
			log.Infof("Discovered AutoNAT peer %s", p.Pretty())
			as.mx.Lock()
			as.peers[p] = as.host.Peerstore().Addrs(p)
			as.mx.Unlock()
		}
	}()
}

func (as *AmbientAutoNAT) Disconnected(net inet.Network, c inet.Conn) {}
