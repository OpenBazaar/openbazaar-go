package transport

import (
	"fmt"

	manet "gx/ipfs/QmYp8PC6b9M3UY4awdHkdRUim68KpGSmRmz27bANHteen6/go-multiaddr-net"
	mautp "gx/ipfs/QmYp8PC6b9M3UY4awdHkdRUim68KpGSmRmz27bANHteen6/go-multiaddr-net/utp"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	utp "gx/ipfs/QmaxuoSyFFzKgrujaburn4j3MQWFQu8ASqVSwrMER6Mk9L/utp"
	mafmt "gx/ipfs/QmeLQ13LftT9XhNn22piZc3GP56fGqhijuL5Y8KdUaRn1g/mafmt"
)

type FallbackDialer struct {
	madialer manet.Dialer
}

func (fbd *FallbackDialer) Matches(a ma.Multiaddr) bool {
	return mafmt.TCP.Matches(a) || mafmt.UTP.Matches(a)
}

func (fbd *FallbackDialer) Dial(a ma.Multiaddr) (Conn, error) {
	if mafmt.TCP.Matches(a) {
		return fbd.tcpDial(a)
	}
	if mafmt.UTP.Matches(a) {
		return fbd.utpDial(a)
	}
	return nil, fmt.Errorf("cannot dial %s with fallback dialer", a)
}

func (fbd *FallbackDialer) tcpDial(raddr ma.Multiaddr) (Conn, error) {
	var c manet.Conn
	var err error
	c, err = fbd.madialer.Dial(raddr)

	if err != nil {
		return nil, err
	}

	return &connWrap{
		Conn: c,
	}, nil
}

func (fbd *FallbackDialer) utpDial(raddr ma.Multiaddr) (Conn, error) {
	_, addr, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}

	con, err := utp.Dial(addr)
	if err != nil {
		return nil, err
	}

	mnc, err := manet.WrapNetConn(&mautp.Conn{Conn: con})
	if err != nil {
		return nil, err
	}

	return &connWrap{
		Conn: mnc,
	}, nil
}
