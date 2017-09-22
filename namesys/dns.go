package namesys

import (
	"context"
	"errors"
	isd "gx/ipfs/QmZmmuAXgX73UQmX1jRKjTGmjzq24Jinqkq8vzkBtno4uX/go-is-domain"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"net"
)

type LookupTXTFunc func(name string) (txt []string, err error)

// DNSResolver implements a Resolver on DNS domains
type DNSResolver struct {
	lookupTXT LookupTXTFunc
}

// NewDNSResolver constructs a name resolver using DNS TXT records.
func NewDNSResolver() Resolver {
	return &DNSResolver{lookupTXT: net.LookupTXT}
}

// Resolve implements Resolver.
func (r *DNSResolver) Resolve(ctx context.Context, name string) (peer.ID, error) {
	return r.resolveOnce(ctx, name)
}

// Resolve implements Resolver.
func (r *DNSResolver) Domains() []string {
	return []string{"dns"}
}

type lookupRes struct {
	pid   peer.ID
	error error
}

// resolveOnce implements resolver.
// TXT records for a given domain name should contain a b58
// encoded multihash.
func (r *DNSResolver) resolveOnce(ctx context.Context, name string) (peer.ID, error) {
	if !isd.IsDomain(name) {
		return "", errors.New("not a valid domain name")
	}

	rootChan := make(chan lookupRes, 1)
	go workDomain(r, name, rootChan)

	var rootRes lookupRes
	var pid peer.ID
	select {
	case rootRes = <-rootChan:
	case <-ctx.Done():
		return pid, ctx.Err()
	}

	if rootRes.error == nil {
		pid = rootRes.pid
	}
	return pid, rootRes.error
}

func workDomain(r *DNSResolver, name string, res chan lookupRes) {
	txt, err := r.lookupTXT(name)
	var pid peer.ID
	if err != nil {
		// Error is != nil
		res <- lookupRes{pid, err}
		return
	}

	for _, t := range txt {
		pid, err = parseEntry(t)
		if err == nil {
			res <- lookupRes{pid, nil}
			return
		}
	}
	res <- lookupRes{pid, ErrResolveFailed}
}

func parseEntry(txt string) (peer.ID, error) {
	return peer.IDB58Decode(txt)
}
