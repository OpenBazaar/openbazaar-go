package namesys

import (
	"context"
	"errors"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"strings"
	"time"
)

// ErrResolveFailed signals an error when attempting to resolve.
var ErrResolveFailed = errors.New("Could not resolve name.")

// ErrNoResolver signals no resolver exists for the specified domain.
var ErrNoResolver = errors.New("No resover for domain.")

const cacheTTL = time.Minute

type NameSystem struct {
	resolvers map[string]Resolver
	cache     map[string]cachedName
}

type cachedName struct {
	id         peer.ID
	expiration time.Time
}

func NewNameSystem(resolvers []Resolver) (*NameSystem, error) {
	n := &NameSystem{
		resolvers: make(map[string]Resolver),
		cache:     make(map[string]cachedName),
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

	cn, ok := n.cache[name]
	if ok {
		if cn.expiration.Before(time.Now()) {
			return cn.id, nil
		}
	}

	r, ok := n.resolvers[ext]
	if ok {
		pid, err = r.Resolve(ctx, name)
		if err != nil {
			return pid, err
		}
		n.cache[name] = cachedName{pid, time.Now().Add(cacheTTL)}
		return pid, nil
	}

	dnsResolver, ok := n.resolvers["dns"]
	if ok {
		pid, err = dnsResolver.Resolve(ctx, name)
		if err != nil {
			return pid, err
		}
		n.cache[name] = cachedName{pid, time.Now().Add(cacheTTL)}
		return pid, nil
	}
	return pid, ErrNoResolver
}
