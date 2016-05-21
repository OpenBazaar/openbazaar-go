// package multistream implements a peerstream transport using
// go-multistream to select the underlying stream muxer
package multistream

import (
	"net"

	mss "gx/ipfs/Qmf91yhgRLo2dhhbc5zZ7TxjMaR1oxaWaoc9zRZdi1kU4a/go-multistream"

	smux "gx/ipfs/Qmb1US8uyZeEpMyc56wVZy2cDFdQjNFojAUYVCoo9ieTqp/go-stream-muxer"
)

type Transport struct {
	mux *mss.MultistreamMuxer

	tpts map[string]smux.Transport

	OrderPreference []string
}

func NewBlankTransport() *Transport {
	return &Transport{
		mux:  mss.NewMultistreamMuxer(),
		tpts: make(map[string]smux.Transport),
	}
}

func (t *Transport) AddTransport(path string, tpt smux.Transport) {
	t.mux.AddHandler(path, nil)
	t.tpts[path] = tpt
	t.OrderPreference = append(t.OrderPreference, path)
}

func (t *Transport) NewConn(nc net.Conn, isServer bool) (smux.Conn, error) {
	var proto string
	if isServer {
		selected, _, err := t.mux.Negotiate(nc)
		if err != nil {
			return nil, err
		}
		proto = selected
	} else {
		selected, err := mss.SelectOneOf(t.OrderPreference, nc)
		if err != nil {
			return nil, err
		}
		proto = selected
	}

	tpt := t.tpts[proto]

	return tpt.NewConn(nc, isServer)
}
