package host

import pstore "gx/ipfs/QmaCTz9RkrU13bm9kMB54f7atgqM4qkjDZpRwRoJiWXEqs/go-libp2p-peerstore"

// PeerInfoFromHost returns a PeerInfo struct with the Host's ID and all of its Addrs.
func PeerInfoFromHost(h Host) *pstore.PeerInfo {
	return &pstore.PeerInfo{
		ID:    h.ID(),
		Addrs: h.Addrs(),
	}
}
