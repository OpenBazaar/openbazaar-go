package relay

import (
	"context"
	"time"

	basic "gx/ipfs/QmRxk6AUaGaKCfzS1xSNRojiAPd7h2ih8GuCdjJBF3Y6GK/go-libp2p/p2p/host/basic"

	ma "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	discovery "gx/ipfs/QmWXhsJTd4eTVAy9n8mDYiFmcMv1VHJ73qGkkeDHZfDhui/go-libp2p-discovery"
	host "gx/ipfs/QmYrWiWM4qtrnCeT3R14jY3ZZyirDNJgwK57q4qFYePgbd/go-libp2p-host"
)

var (
	AdvertiseBootDelay = 30 * time.Second
)

// RelayHost is a Host that provides Relay services.
type RelayHost struct {
	*basic.BasicHost
	advertise discovery.Advertiser
	addrsF    basic.AddrsFactory
}

// New constructs a new RelayHost
func NewRelayHost(ctx context.Context, bhost *basic.BasicHost, advertise discovery.Advertiser) *RelayHost {
	h := &RelayHost{
		BasicHost: bhost,
		addrsF:    bhost.AddrsFactory,
		advertise: advertise,
	}
	bhost.AddrsFactory = h.hostAddrs
	go func() {
		select {
		case <-time.After(AdvertiseBootDelay):
			discovery.Advertise(ctx, advertise, RelayRendezvous)
		case <-ctx.Done():
		}
	}()
	return h
}

func (h *RelayHost) hostAddrs(addrs []ma.Multiaddr) []ma.Multiaddr {
	return filterUnspecificRelay(h.addrsF(addrs))
}

var _ host.Host = (*RelayHost)(nil)
