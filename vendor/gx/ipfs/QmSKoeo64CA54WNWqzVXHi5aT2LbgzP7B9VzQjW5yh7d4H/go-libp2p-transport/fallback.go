package transport

import (
	"context"
	"fmt"

	mafmt "gx/ipfs/QmSkUKdsSrEo1v28Y3NJ7vQT7jmbxg87g8ucbhctwHEqb4/mafmt"
	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
	manet "gx/ipfs/Qmf1Gq7N45Rpuw7ev47uWgH6dLPtdnvcMRNPkVBwqjLJg2/go-multiaddr-net"
)

type FallbackDialer struct {
	madialer manet.Dialer
}

func (fbd *FallbackDialer) Matches(a ma.Multiaddr) bool {
	return mafmt.TCP.Matches(a)
}

func (fbd *FallbackDialer) Dial(a ma.Multiaddr) (Conn, error) {
	return fbd.DialContext(context.Background(), a)
}

func (fbd *FallbackDialer) DialContext(ctx context.Context, a ma.Multiaddr) (Conn, error) {
	if mafmt.TCP.Matches(a) {
		return fbd.tcpDial(ctx, a)
	}
	return nil, fmt.Errorf("cannot dial %s with fallback dialer", a)
}

func (fbd *FallbackDialer) tcpDial(ctx context.Context, raddr ma.Multiaddr) (Conn, error) {
	var c manet.Conn
	var err error
	c, err = fbd.madialer.DialContext(ctx, raddr)

	if err != nil {
		return nil, err
	}

	return &ConnWrap{
		Conn: c,
	}, nil
}

var _ Dialer = (*FallbackDialer)(nil)
