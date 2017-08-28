package namesys

import (
	"context"
	"errors"
	"gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
	"strings"
)

// ErrResolveFailed signals an error when attempting to resolve.
var ErrResolveFailed = errors.New("Could not resolve name.")

// ErrNoResolver signals no resolver exists for the specified domain.
var ErrNoResolver = errors.New("No resover for domain.")

type NameSystem struct {
	resolvers map[string]Resolver
}

func NewNameSystem(resolvers []Resolver) (*NameSystem, error) {
	n := &NameSystem{
		resolvers: make(map[string]Resolver),
	}
	for _, r := range resolvers {
		for _, domain := range r.Domains() {
			n.resolvers[domain] = r
		}
	}
	return n, nil
}

func (n *NameSystem) Resolve(ctx context.Context, name string) (pid peer.ID, err error) {
	pid, err = peer.IDB58Decode(name)
	if err == nil {
		return pid, nil
	}

	split := strings.Split(name, ".")
	ext := split[len(split)-1]
	r, ok := n.resolvers[ext]
	if ok {
		return r.Resolve(ctx, name)
	}

	dnsResolver, ok := n.resolvers["dns"]
	if ok {
		return dnsResolver.Resolve(ctx, name)
	}
	return pid, ErrNoResolver
}
