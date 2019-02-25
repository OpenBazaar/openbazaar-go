package pstoremem

import pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"

// NewPeerstore creates an in-memory threadsafe collection of peers.
func NewPeerstore() pstore.Peerstore {
	return pstore.NewPeerstore(
		NewKeyBook(),
		NewAddrBook(),
		NewPeerMetadata())
}
