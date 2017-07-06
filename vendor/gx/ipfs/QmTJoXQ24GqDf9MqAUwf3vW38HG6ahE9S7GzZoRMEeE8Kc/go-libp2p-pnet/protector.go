package pnet

import (
	"io"

	ipnet "gx/ipfs/QmPsBptED6X43GYg3347TAUruN3UfsAhaGTP9xbinYX7uf/go-libp2p-interface-pnet"
	tconn "gx/ipfs/QmSKoeo64CA54WNWqzVXHi5aT2LbgzP7B9VzQjW5yh7d4H/go-libp2p-transport"
)

var _ ipnet.Protector = (*protector)(nil)

// NewProtector creates ipnet.Protector instance from a io.Reader stream
// that should include Multicodec encoded V1 PSK.
func NewProtector(input io.Reader) (ipnet.Protector, error) {
	psk, err := decodeV1PSKKey(input)
	if err != nil {
		return nil, err
	}
	f := fingerprint(psk)

	return &protector{
		psk:         psk,
		fingerprint: f,
	}, nil
}

type protector struct {
	psk         *[32]byte
	fingerprint []byte
}

func (p protector) Protect(in tconn.Conn) (tconn.Conn, error) {
	return newPSKConn(p.psk, in)
}
func (p protector) Fingerprint() []byte {
	return p.fingerprint
}
