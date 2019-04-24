package autonat

import (
	"context"
	"sync"
	"time"

	pb "gx/ipfs/QmZjvy5eqc1T9VbFe5KBiDryUJSRNbSbHZD7JMc6CeUXuu/go-libp2p-autonat/pb"

	libp2p "gx/ipfs/QmRxk6AUaGaKCfzS1xSNRojiAPd7h2ih8GuCdjJBF3Y6GK/go-libp2p"
	ma "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	inet "gx/ipfs/QmY3ArotKMKaL7YGfbQfyDrib6RVraLqZYWXZvVgZktBxp/go-libp2p-net"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	host "gx/ipfs/QmYrWiWM4qtrnCeT3R14jY3ZZyirDNJgwK57q4qFYePgbd/go-libp2p-host"
	autonat "gx/ipfs/QmZjvy5eqc1T9VbFe5KBiDryUJSRNbSbHZD7JMc6CeUXuu/go-libp2p-autonat"
	pstore "gx/ipfs/QmaCTz9RkrU13bm9kMB54f7atgqM4qkjDZpRwRoJiWXEqs/go-libp2p-peerstore"
	manet "gx/ipfs/Qmc85NSvmSG4Frn9Vb2cBc1rMyULH6D3TNVEfCzSKoUpip/go-multiaddr-net"
	ggio "gx/ipfs/QmddjPSGZb3ieihSseFeCfVRpZzcqczPNsD2DvarSwnjJB/gogo-protobuf/io"
)

const P_CIRCUIT = 290

var (
	AutoNATServiceDialTimeout   = 42 * time.Second
	AutoNATServiceResetInterval = 1 * time.Minute

	AutoNATServiceThrottle = 3
)

// AutoNATService provides NAT autodetection services to other peers
type AutoNATService struct {
	ctx    context.Context
	dialer host.Host

	// rate limiter
	mx   sync.Mutex
	reqs map[peer.ID]int
}

// NewAutoNATService creates a new AutoNATService instance attached to a host
func NewAutoNATService(ctx context.Context, h host.Host, opts ...libp2p.Option) (*AutoNATService, error) {
	opts = append(opts, libp2p.NoListenAddrs)
	dialer, err := libp2p.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	as := &AutoNATService{
		ctx:    ctx,
		dialer: dialer,
		reqs:   make(map[peer.ID]int),
	}
	h.SetStreamHandler(autonat.AutoNATProto, as.handleStream)

	go as.resetRateLimiter()

	return as, nil
}

func (as *AutoNATService) handleStream(s inet.Stream) {
	defer s.Close()

	pid := s.Conn().RemotePeer()
	log.Debugf("New stream from %s", pid.Pretty())

	r := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
	w := ggio.NewDelimitedWriter(s)

	var req pb.Message
	var res pb.Message

	err := r.ReadMsg(&req)
	if err != nil {
		log.Debugf("Error reading message from %s: %s", pid.Pretty(), err.Error())
		s.Reset()
		return
	}

	t := req.GetType()
	if t != pb.Message_DIAL {
		log.Debugf("Unexpected message from %s: %s (%d)", pid.Pretty(), t.String(), t)
		s.Reset()
		return
	}

	dr := as.handleDial(pid, s.Conn().RemoteMultiaddr(), req.GetDial().GetPeer())
	res.Type = pb.Message_DIAL_RESPONSE.Enum()
	res.DialResponse = dr

	err = w.WriteMsg(&res)
	if err != nil {
		log.Debugf("Error writing response to %s: %s", pid.Pretty(), err.Error())
		s.Reset()
		return
	}
}

func (as *AutoNATService) handleDial(p peer.ID, obsaddr ma.Multiaddr, mpi *pb.Message_PeerInfo) *pb.Message_DialResponse {
	if mpi == nil {
		return newDialResponseError(pb.Message_E_BAD_REQUEST, "missing peer info")
	}

	mpid := mpi.GetId()
	if mpid != nil {
		mp, err := peer.IDFromBytes(mpid)
		if err != nil {
			return newDialResponseError(pb.Message_E_BAD_REQUEST, "bad peer id")
		}

		if mp != p {
			return newDialResponseError(pb.Message_E_BAD_REQUEST, "peer id mismatch")
		}
	}

	addrs := make([]ma.Multiaddr, 0)
	seen := make(map[string]struct{})

	// add observed addr to the list of addresses to dial
	if !as.skipDial(obsaddr) {
		addrs = append(addrs, obsaddr)
		seen[obsaddr.String()] = struct{}{}
	}

	for _, maddr := range mpi.GetAddrs() {
		addr, err := ma.NewMultiaddrBytes(maddr)
		if err != nil {
			log.Debugf("Error parsing multiaddr: %s", err.Error())
			continue
		}

		if as.skipDial(addr) {
			continue
		}

		str := addr.String()
		_, ok := seen[str]
		if ok {
			continue
		}

		addrs = append(addrs, addr)
		seen[str] = struct{}{}
	}

	if len(addrs) == 0 {
		return newDialResponseError(pb.Message_E_DIAL_ERROR, "no dialable addresses")
	}

	return as.doDial(pstore.PeerInfo{ID: p, Addrs: addrs})
}

func (as *AutoNATService) skipDial(addr ma.Multiaddr) bool {
	// skip relay addresses
	_, err := addr.ValueForProtocol(P_CIRCUIT)
	if err == nil {
		return true
	}

	// skip private network (unroutable) addresses
	if !manet.IsPublicAddr(addr) {
		return true
	}

	return false
}

func (as *AutoNATService) doDial(pi pstore.PeerInfo) *pb.Message_DialResponse {
	// rate limit check
	as.mx.Lock()
	count := as.reqs[pi.ID]
	if count >= AutoNATServiceThrottle {
		as.mx.Unlock()
		return newDialResponseError(pb.Message_E_DIAL_REFUSED, "too many dials")
	}
	as.reqs[pi.ID] = count + 1
	as.mx.Unlock()

	ctx, cancel := context.WithTimeout(as.ctx, AutoNATServiceDialTimeout)
	defer cancel()

	err := as.dialer.Connect(ctx, pi)
	if err != nil {
		log.Debugf("error dialing %s: %s", pi.ID.Pretty(), err.Error())
		// wait for the context to timeout to avoid leaking timing information
		// this renders the service ineffective as a port scanner
		<-ctx.Done()
		return newDialResponseError(pb.Message_E_DIAL_ERROR, "dial failed")
	}

	conns := as.dialer.Network().ConnsToPeer(pi.ID)
	if len(conns) == 0 {
		log.Errorf("supposedly connected to %s, but no connection to peer", pi.ID.Pretty())
		return newDialResponseError(pb.Message_E_INTERNAL_ERROR, "internal service error")
	}

	ra := conns[0].RemoteMultiaddr()
	as.dialer.Network().ClosePeer(pi.ID)
	return newDialResponseOK(ra)
}

func (as *AutoNATService) resetRateLimiter() {
	ticker := time.NewTicker(AutoNATServiceResetInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			as.mx.Lock()
			as.reqs = make(map[peer.ID]int)
			as.mx.Unlock()

		case <-as.ctx.Done():
			return
		}
	}
}
