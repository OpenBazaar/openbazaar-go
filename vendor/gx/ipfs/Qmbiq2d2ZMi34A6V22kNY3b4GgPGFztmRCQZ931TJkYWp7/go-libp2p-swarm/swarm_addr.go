package swarm

import (
	ma "gx/ipfs/QmUAQaWbKxGCUTuoQVvvicbQNZ9APF5pDGWyAZSe93AtKH/go-multiaddr"
	addrutil "gx/ipfs/QmVDnc2zvyQm8LhT72n22THcshvH7j3qPMnhvjerQER62T/go-addr-util"
	iconn "gx/ipfs/QmctX4TY6jXtpfeDiwMGoB4qVTBGDnz7T7r22CwQSzTgwt/go-libp2p-interface-conn"
)

// ListenAddresses returns a list of addresses at which this swarm listens.
func (s *Swarm) ListenAddresses() []ma.Multiaddr {
	listeners := s.swarm.Listeners()
	addrs := make([]ma.Multiaddr, 0, len(listeners))
	for _, l := range listeners {
		if l2, ok := l.NetListener().(iconn.Listener); ok {
			addrs = append(addrs, l2.Multiaddr())
		}
	}
	return addrs
}

// InterfaceListenAddresses returns a list of addresses at which this swarm
// listens. It expands "any interface" addresses (/ip4/0.0.0.0, /ip6/::) to
// use the known local interfaces.
func (s *Swarm) InterfaceListenAddresses() ([]ma.Multiaddr, error) {
	return addrutil.ResolveUnspecifiedAddresses(s.ListenAddresses(), nil)
}
