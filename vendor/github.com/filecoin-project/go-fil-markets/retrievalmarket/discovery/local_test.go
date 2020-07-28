package discovery_test

import (
	"testing"

	specst "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/discovery"
	"github.com/filecoin-project/go-fil-markets/shared_testutil"
)

func TestLocal_AddPeer(t *testing.T) {

	peer1 := retrievalmarket.RetrievalPeer{
		Address: specst.NewIDAddr(t, 1),
		ID:      peer.NewPeerRecord().PeerID,
	}
	peer2 := retrievalmarket.RetrievalPeer{
		Address: specst.NewIDAddr(t, 2),
		ID:      peer.NewPeerRecord().PeerID,
	}
	testCases := []struct {
		name      string
		peers2add []retrievalmarket.RetrievalPeer
		expPeers  []retrievalmarket.RetrievalPeer
	}{
		{
			name:      "can add 3 peers",
			peers2add: []retrievalmarket.RetrievalPeer{peer1, peer2},
			expPeers:  []retrievalmarket.RetrievalPeer{peer1, peer2},
		},
		{
			name:      "can add same peer without duping",
			peers2add: []retrievalmarket.RetrievalPeer{peer1, peer1},
			expPeers:  []retrievalmarket.RetrievalPeer{peer1},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ds := datastore.NewMapDatastore()
			l := discovery.NewLocal(ds)
			payloadCID := shared_testutil.GenerateCids(1)[0]
			for _, testpeer := range tc.peers2add {
				require.NoError(t, l.AddPeer(payloadCID, testpeer))
			}
			actualPeers, err := l.GetPeers(payloadCID)
			require.NoError(t, err)
			assert.Equal(t, len(tc.expPeers), len(actualPeers))
			assert.Equal(t, tc.expPeers[0], actualPeers[0])
		})
	}
}
