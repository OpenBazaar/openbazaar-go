package relay

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	basic "gx/ipfs/QmRxk6AUaGaKCfzS1xSNRojiAPd7h2ih8GuCdjJBF3Y6GK/go-libp2p/p2p/host/basic"

	ma "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	discovery "gx/ipfs/QmWXhsJTd4eTVAy9n8mDYiFmcMv1VHJ73qGkkeDHZfDhui/go-libp2p-discovery"
	inet "gx/ipfs/QmY3ArotKMKaL7YGfbQfyDrib6RVraLqZYWXZvVgZktBxp/go-libp2p-net"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	host "gx/ipfs/QmYrWiWM4qtrnCeT3R14jY3ZZyirDNJgwK57q4qFYePgbd/go-libp2p-host"
	routing "gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing"
	_ "gx/ipfs/QmZBfqr863PYD7BKbmCFSNmzsqYmtr2DKgzubsQaiTQkMc/go-libp2p-circuit"
	autonat "gx/ipfs/QmZjvy5eqc1T9VbFe5KBiDryUJSRNbSbHZD7JMc6CeUXuu/go-libp2p-autonat"
	pstore "gx/ipfs/QmaCTz9RkrU13bm9kMB54f7atgqM4qkjDZpRwRoJiWXEqs/go-libp2p-peerstore"
	manet "gx/ipfs/Qmc85NSvmSG4Frn9Vb2cBc1rMyULH6D3TNVEfCzSKoUpip/go-multiaddr-net"
)

const (
	RelayRendezvous = "/libp2p/relay"
)

var (
	DesiredRelays = 3

	BootDelay = 60 * time.Second

	unspecificRelay ma.Multiaddr
)

func init() {
	var err error
	unspecificRelay, err = ma.NewMultiaddr("/p2p-circuit")
	if err != nil {
		panic(err)
	}
}

// AutoRelayHost is a Host that uses relays for connectivity when a NAT is detected.
type AutoRelayHost struct {
	*basic.BasicHost
	discover discovery.Discoverer
	router   routing.PeerRouting
	autonat  autonat.AutoNAT
	addrsF   basic.AddrsFactory

	disconnect chan struct{}

	mx     sync.Mutex
	relays map[peer.ID]pstore.PeerInfo
	addrs  []ma.Multiaddr
}

func NewAutoRelayHost(ctx context.Context, bhost *basic.BasicHost, discover discovery.Discoverer, router routing.PeerRouting) *AutoRelayHost {
	h := &AutoRelayHost{
		BasicHost:  bhost,
		discover:   discover,
		router:     router,
		addrsF:     bhost.AddrsFactory,
		relays:     make(map[peer.ID]pstore.PeerInfo),
		disconnect: make(chan struct{}, 1),
	}
	h.autonat = autonat.NewAutoNAT(ctx, bhost, h.baseAddrs)
	bhost.AddrsFactory = h.hostAddrs
	bhost.Network().Notify(h)
	go h.background(ctx)
	return h
}

func (h *AutoRelayHost) hostAddrs(addrs []ma.Multiaddr) []ma.Multiaddr {
	h.mx.Lock()
	defer h.mx.Unlock()
	if h.addrs != nil && h.autonat.Status() == autonat.NATStatusPrivate {
		return h.addrs
	} else {
		return filterUnspecificRelay(h.addrsF(addrs))
	}
}

func (h *AutoRelayHost) baseAddrs() []ma.Multiaddr {
	return filterUnspecificRelay(h.addrsF(h.AllAddrs()))
}

func (h *AutoRelayHost) background(ctx context.Context) {
	select {
	case <-time.After(autonat.AutoNATBootDelay + BootDelay):
	case <-ctx.Done():
		return
	}

	for {
		wait := autonat.AutoNATRefreshInterval
		switch h.autonat.Status() {
		case autonat.NATStatusUnknown:
			wait = autonat.AutoNATRetryInterval
		case autonat.NATStatusPublic:
		case autonat.NATStatusPrivate:
			h.findRelays(ctx)
		}

		select {
		case <-h.disconnect:
			// invalidate addrs
			h.mx.Lock()
			h.addrs = nil
			h.mx.Unlock()
		case <-time.After(wait):
		case <-ctx.Done():
			return
		}
	}
}

func (h *AutoRelayHost) findRelays(ctx context.Context) {
	log.Debugf("findRelays entered")
	h.mx.Lock()
	if len(h.relays) >= DesiredRelays {
		h.mx.Unlock()
		return
	}
	need := DesiredRelays - len(h.relays)
	h.mx.Unlock()

	limit := 20
	if need > limit/2 {
		limit = 2 * need
	}

	dctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	log.Debugf("find peers for relay")
	pis, err := discovery.FindPeers(dctx, h.discover, RelayRendezvous, limit)
	cancel()
	if err != nil {
		log.Debugf("error discovering relays: %s", err.Error())
		return
	}

	log.Debugf("select peers for relay")
	pis = h.selectRelays(pis)

	update := 0

	for _, pi := range pis {
		h.mx.Lock()
		if _, ok := h.relays[pi.ID]; ok {
			h.mx.Unlock()
			continue
		}
		h.mx.Unlock()

		cctx, cancel := context.WithTimeout(ctx, 600*time.Second)

		if len(pi.Addrs) == 0 {
			log.Debugf("no addrs for peer %s, getting addrs for relay", pi.ID)
			pi, err = h.router.FindPeer(cctx, pi.ID)
			if err != nil {
				log.Debugf("error finding relay peer %s: %s", pi.ID, err.Error())
				cancel()
				continue
			}
		}

		log.Debugf("connecting to peer %s for relay", pi.ID)
		err = h.Connect(cctx, pi)
		cancel()
		if err != nil {
			log.Debugf("error connecting to relay %s: %s", pi.ID, err.Error())
			continue
		}

		log.Debugf("connected to relay %s", pi.ID)
		h.mx.Lock()
		h.relays[pi.ID] = pi
		h.mx.Unlock()

		// tag the connection as very important
		log.Debugf("connected! tag peer %s as 'relay'", pi.ID)
		h.ConnManager().TagPeer(pi.ID, "relay", 42)

		update++
		need--
		if need == 0 {
			break
		}
	}

	if update > 0 || h.addrs == nil {
		h.updateAddrs()
	}
}

func (h *AutoRelayHost) selectRelays(pis []pstore.PeerInfo) []pstore.PeerInfo {
	// TODO better relay selection strategy; this just selects random relays
	//      but we should probably use ping latency as the selection metric
	shuffleRelays(pis)
	return pis
}

func (h *AutoRelayHost) updateAddrs() {
	h.doUpdateAddrs()
	h.PushIdentify()
}

// This function updates our NATed advertised addrs (h.addrs)
// The public addrs are rewritten so that they only retain the public IP part; they
// become undialable but are useful as a hint to the dialer to determine whether or not
// to dial private addrs.
// The non-public addrs are included verbatim so that peers behind the same NAT/firewall
// can still dial us directly.
// On top of those, we add the relay-specific addrs for the relays to which we are
// connected. For each non-private relay addr, we encapsulate the p2p-circuit addr
// through which we can be dialed.
func (h *AutoRelayHost) doUpdateAddrs() {
	log.Debugf("updating relay addrs")
	defer log.Debugf("done updating relay addrs")
	h.mx.Lock()
	defer h.mx.Unlock()

	addrs := h.baseAddrs()
	raddrs := make([]ma.Multiaddr, 0, len(addrs)+len(h.relays))

	// remove our public addresses from the list
	for _, addr := range addrs {
		if manet.IsPublicAddr(addr) {
			continue
		}
		raddrs = append(raddrs, addr)
	}

	// add relay specific addrs to the list
	for _, pi := range h.relays {
		circuit, err := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s/p2p-circuit", pi.ID.Pretty()))
		if err != nil {
			panic(err)
		}

		for _, addr := range pi.Addrs {
			if !manet.IsPrivateAddr(addr) {
				pub := addr.Encapsulate(circuit)
				raddrs = append(raddrs, pub)
			}
		}
	}

	h.addrs = raddrs
}

func filterUnspecificRelay(addrs []ma.Multiaddr) []ma.Multiaddr {
	res := make([]ma.Multiaddr, 0, len(addrs))
	for _, addr := range addrs {
		if addr.Equal(unspecificRelay) {
			continue
		}
		res = append(res, addr)
	}
	return res
}

func shuffleRelays(pis []pstore.PeerInfo) {
	for i := range pis {
		j := rand.Intn(i + 1)
		pis[i], pis[j] = pis[j], pis[i]
	}
}

func containsAddr(lst []ma.Multiaddr, addr ma.Multiaddr) bool {
	for _, xaddr := range lst {
		if xaddr.Equal(addr) {
			return true
		}
	}
	return false
}

// notify
func (h *AutoRelayHost) Listen(inet.Network, ma.Multiaddr)      {}
func (h *AutoRelayHost) ListenClose(inet.Network, ma.Multiaddr) {}
func (h *AutoRelayHost) Connected(inet.Network, inet.Conn)      {}

func (h *AutoRelayHost) Disconnected(_ inet.Network, c inet.Conn) {
	p := c.RemotePeer()
	h.mx.Lock()
	defer h.mx.Unlock()
	if _, ok := h.relays[p]; ok {
		delete(h.relays, p)
		select {
		case h.disconnect <- struct{}{}:
		default:
		}
	}
}

func (h *AutoRelayHost) OpenedStream(inet.Network, inet.Stream) {}
func (h *AutoRelayHost) ClosedStream(inet.Network, inet.Stream) {}

var _ host.Host = (*AutoRelayHost)(nil)
