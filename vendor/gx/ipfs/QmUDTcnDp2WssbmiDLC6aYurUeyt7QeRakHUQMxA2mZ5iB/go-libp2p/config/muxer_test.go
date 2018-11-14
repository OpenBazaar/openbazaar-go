package config

import (
	"context"
	"testing"

	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	bhost "gx/ipfs/QmUDTcnDp2WssbmiDLC6aYurUeyt7QeRakHUQMxA2mZ5iB/go-libp2p/p2p/host/basic"
	swarmt "gx/ipfs/QmVHhT8NxtApPTndiZPe4JNGNUxGWtJe3ebyxtRz4HnbEp/go-libp2p-swarm/testing"
	mux "gx/ipfs/QmY9JXR3FupnYAYJWK9aMr9bCpqWKcToQ1tz8DVGTrHpHw/go-stream-muxer"
	host "gx/ipfs/QmdJfsSbKSZnMkfZ1kpopiyB9i3Hd6cp8VKWZmtWPa7Moc/go-libp2p-host"
	yamux "gx/ipfs/Qmdps3CYh5htGQSrPvzg5PHouVexLmtpbuLCqc4vuej8PC/go-smux-yamux"
)

func TestMuxerSimple(t *testing.T) {
	// single
	_, err := MuxerConstructor(func(_ peer.ID) mux.Transport { return nil })
	if err != nil {
		t.Fatal(err)
	}
}

func TestMuxerByValue(t *testing.T) {
	_, err := MuxerConstructor(yamux.DefaultTransport)
	if err != nil {
		t.Fatal(err)
	}
}
func TestMuxerDuplicate(t *testing.T) {
	_, err := MuxerConstructor(func(_ peer.ID, _ peer.ID) mux.Transport { return nil })
	if err != nil {
		t.Fatal(err)
	}
}

func TestMuxerError(t *testing.T) {
	_, err := MuxerConstructor(func() (mux.Transport, error) { return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
}

func TestMuxerBadTypes(t *testing.T) {
	for i, f := range []interface{}{
		func() error { return nil },
		func() string { return "" },
		func() {},
		func(string) mux.Transport { return nil },
		func(string) (mux.Transport, error) { return nil, nil },
		nil,
		"testing",
	} {

		if _, err := MuxerConstructor(f); err == nil {
			t.Fatalf("constructor %d with type %T should have failed", i, f)
		}
	}
}

func TestCatchDuplicateTransportsMuxer(t *testing.T) {
	ctx := context.Background()
	h := bhost.New(swarmt.GenSwarm(t, ctx))
	yamuxMuxer, err := MuxerConstructor(yamux.DefaultTransport)
	if err != nil {
		t.Fatal(err)
	}

	var tests = map[string]struct {
		h             host.Host
		transports    []MsMuxC
		expectedError string
	}{
		"no duplicate transports": {
			h:             h,
			transports:    []MsMuxC{MsMuxC{yamuxMuxer, "yamux"}},
			expectedError: "",
		},
		"duplicate transports": {
			h: h,
			transports: []MsMuxC{
				MsMuxC{yamuxMuxer, "yamux"},
				MsMuxC{yamuxMuxer, "yamux"},
			},
			expectedError: "duplicate muxer transport: yamux",
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			_, err = makeMuxer(test.h, test.transports)
			if err != nil {
				if err.Error() != test.expectedError {
					t.Errorf(
						"\nexpected: [%v]\nactual:   [%v]\n",
						test.expectedError,
						err,
					)
				}
			}
		})
	}
}
