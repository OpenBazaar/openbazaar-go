package config

import (
	"fmt"

	msmux "gx/ipfs/QmUWkEreTZdTxUVDfpQ2fLywJh6dinfEYahEoBDTMQ2hks/go-smux-multistream"
	mux "gx/ipfs/QmVtV1y2e8W4eQgzsP6qfSpCCZ6zWYE4m6NzJjB7iswwrT/go-stream-muxer"
	host "gx/ipfs/QmYrWiWM4qtrnCeT3R14jY3ZZyirDNJgwK57q4qFYePgbd/go-libp2p-host"
)

// MuxC is a stream multiplex transport constructor
type MuxC func(h host.Host) (mux.Transport, error)

// MsMuxC is a tuple containing a multiplex transport constructor and a protocol
// ID.
type MsMuxC struct {
	MuxC
	ID string
}

var muxArgTypes = newArgTypeSet(hostType, networkType, peerIDType, pstoreType)

// MuxerConstructor creates a multiplex constructor from the passed parameter
// using reflection.
func MuxerConstructor(m interface{}) (MuxC, error) {
	// Already constructed?
	if t, ok := m.(mux.Transport); ok {
		return func(_ host.Host) (mux.Transport, error) {
			return t, nil
		}, nil
	}

	ctor, err := makeConstructor(m, muxType, muxArgTypes)
	if err != nil {
		return nil, err
	}
	return func(h host.Host) (mux.Transport, error) {
		t, err := ctor(h, nil)
		if err != nil {
			return nil, err
		}
		return t.(mux.Transport), nil
	}, nil
}

func makeMuxer(h host.Host, tpts []MsMuxC) (mux.Transport, error) {
	muxMuxer := msmux.NewBlankTransport()
	transportSet := make(map[string]struct{}, len(tpts))
	for _, tptC := range tpts {
		if _, ok := transportSet[tptC.ID]; ok {
			return nil, fmt.Errorf("duplicate muxer transport: %s", tptC.ID)
		}
		transportSet[tptC.ID] = struct{}{}
	}
	for _, tptC := range tpts {
		tpt, err := tptC.MuxC(h)
		if err != nil {
			return nil, err
		}
		muxMuxer.AddTransport(tptC.ID, tpt)
	}
	return muxMuxer, nil
}
