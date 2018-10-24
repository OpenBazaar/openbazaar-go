package namesys

import (
	"context"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
)

// A Resolver will resolve domain names into PeerIDs that can then been used in IPNS queries.
// OpenBazaar is intended to be agnostic to the underlying name systems as they all have their own
// positives and negatives. New name systems can be added by implementing the Resolver interface
// although only nodes which are updated with the new Resolver will be able to visit such domains.
type Resolver interface {
	// Resolve a domain name into a PeerIDs
	Resolve(ctx context.Context, name string) (peer.ID, error)

	// Returns a list of domains this resolver knows how to resolve
	Domains() []string
}
