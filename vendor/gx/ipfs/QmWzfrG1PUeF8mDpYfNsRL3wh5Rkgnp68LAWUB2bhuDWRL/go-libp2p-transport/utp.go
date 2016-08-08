package transport

import (
	"fmt"
	"net"
	"sync"

	utp "gx/ipfs/QmPSR1zYgWRmTSt6nTGXdeeUhWkq9AdvK4sva4Y2H5daPi/utp"
	manet "gx/ipfs/QmPpRcbNUXauP3zWZ1NJMLWpe4QnmEHrd2ba2D3yqWznw7/go-multiaddr-net"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	mafmt "gx/ipfs/QmeLQ13LftT9XhNn22piZc3GP56fGqhijuL5Y8KdUaRn1g/mafmt"
)

var errIncorrectNetAddr = fmt.Errorf("incorrect network addr conversion")

var utpAddrSpec = &manet.NetCodec{
	ProtocolName:     "utp",
	NetAddrNetworks:  []string{"utp", "utp4", "utp6"},
	ParseNetAddr:     parseUtpNetAddr,
	ConvertMultiaddr: parseUtpMaddr,
}

func init() {
	manet.RegisterNetCodec(utpAddrSpec)
}

type UtpTransport struct {
	sockLock sync.Mutex
	sockets  map[string]*UtpSocket
}

func NewUtpTransport() *UtpTransport {
	return &UtpTransport{
		sockets: make(map[string]*UtpSocket),
	}
}

func (d *UtpTransport) Matches(a ma.Multiaddr) bool {
	return mafmt.UTP.Matches(a)
}

type UtpSocket struct {
	s         *utp.Socket
	laddr     ma.Multiaddr
	transport Transport
}

func (t *UtpTransport) Listen(laddr ma.Multiaddr) (Listener, error) {
	t.sockLock.Lock()
	defer t.sockLock.Unlock()
	s, ok := t.sockets[laddr.String()]
	if ok {
		return s, nil
	}

	ns, err := t.newConn(laddr)
	if err != nil {
		return nil, err
	}

	t.sockets[laddr.String()] = ns
	return ns, nil
}

func (t *UtpTransport) Dialer(laddr ma.Multiaddr, opts ...DialOpt) (Dialer, error) {
	t.sockLock.Lock()
	defer t.sockLock.Unlock()
	s, ok := t.sockets[laddr.String()]
	if ok {
		return s, nil
	}

	ns, err := t.newConn(laddr, opts...)
	if err != nil {
		return nil, err
	}

	t.sockets[laddr.String()] = ns
	return ns, nil
}

func (t *UtpTransport) newConn(addr ma.Multiaddr, opts ...DialOpt) (*UtpSocket, error) {
	network, netaddr, err := manet.DialArgs(addr)
	if err != nil {
		return nil, err
	}

	s, err := utp.NewSocket("udp"+network[3:], netaddr)
	if err != nil {
		return nil, err
	}

	laddr, err := manet.FromNetAddr(s.LocalAddr())
	if err != nil {
		return nil, err
	}

	return &UtpSocket{
		s:         s,
		laddr:     laddr,
		transport: t,
	}, nil
}

func (s *UtpSocket) Dial(raddr ma.Multiaddr) (Conn, error) {
	_, addr, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}

	con, err := s.s.Dial(addr)
	if err != nil {
		return nil, err
	}

	mnc, err := manet.WrapNetConn(con)
	if err != nil {
		return nil, err
	}

	return &connWrap{
		Conn:      mnc,
		transport: s.transport,
	}, nil
}

func (s *UtpSocket) Accept() (Conn, error) {
	c, err := s.s.Accept()
	if err != nil {
		return nil, err
	}

	mnc, err := manet.WrapNetConn(c)
	if err != nil {
		return nil, err
	}

	return &connWrap{
		Conn:      mnc,
		transport: s.transport,
	}, nil
}

func (s *UtpSocket) Matches(a ma.Multiaddr) bool {
	return mafmt.UTP.Matches(a)
}

func (t *UtpSocket) Close() error {
	return t.s.Close()
}

func (t *UtpSocket) Addr() net.Addr {
	return t.s.Addr()
}

func (t *UtpSocket) Multiaddr() ma.Multiaddr {
	return t.laddr
}

var _ Transport = (*UtpTransport)(nil)

func parseUtpNetAddr(a net.Addr) (ma.Multiaddr, error) {
	var udpaddr *net.UDPAddr
	switch a := a.(type) {
	case *utp.Addr:
		udpaddr = a.Child.(*net.UDPAddr)
	case *net.UDPAddr:
		udpaddr = a
	default:
		return nil, fmt.Errorf("was not given a valid utp address")
	}

	// Get IP Addr
	ipm, err := manet.FromIP(udpaddr.IP)
	if err != nil {
		return nil, errIncorrectNetAddr
	}

	// Get UDP Addr
	utpm, err := ma.NewMultiaddr(fmt.Sprintf("/udp/%d/utp", udpaddr.Port))
	if err != nil {
		return nil, errIncorrectNetAddr
	}

	// Encapsulate
	return ipm.Encapsulate(utpm), nil
}

func parseUtpMaddr(maddr ma.Multiaddr) (net.Addr, error) {
	utpbase, err := ma.NewMultiaddr("/utp")
	if err != nil {
		return nil, err
	}

	raw := maddr.Decapsulate(utpbase)

	udpa, err := manet.ToNetAddr(raw)
	if err != nil {
		return nil, err
	}

	return &utp.Addr{udpa}, nil
}
