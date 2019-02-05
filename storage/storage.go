package net

import (
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
)

type OfflineMessagingStorage interface {
	/* This interface provides a pluggable mechanism for implementing a variety
	   of offline message storage solutions. When the app wants to send a message
	   to an offline recipient it will call this store function. Implementations
	   are expected to store the message somewhere accessible to the recipient.
	   The return should be a `Multiadddr` of the storage location. Upon receiving
	   the response to this function a `Pointer` to the location of the message
	   will be placed in the DHT using the recipient's peer ID as the key.

	   Some storage possibilities include:
	   IPFS Seeding -> assumes this node remains online all the time
	   Dropbox -> go dropbox drivers are available
	   Custom Options -> create your own free or paid service.

	   Note all messages are encrypted before passed in here. */
	Store(peerID peer.ID, ciphertext []byte) (ma.Multiaddr, error)
}
