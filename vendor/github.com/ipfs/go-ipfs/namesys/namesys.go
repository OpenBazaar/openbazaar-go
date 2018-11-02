package namesys

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	opts "github.com/ipfs/go-ipfs/namesys/opts"
	path "github.com/ipfs/go-ipfs/path"

	p2phost "gx/ipfs/QmNmJZL7FQySMtE2BQuLMuZg2EB2CLEunJJUSVSc9YnnbV/go-libp2p-host"
	floodsub "gx/ipfs/QmSFihvoND3eDaAYRCeLgLPt62yCPgMZs1NSZmKFEtJQQw/go-libp2p-floodsub"
	routing "gx/ipfs/QmTiWLZ6Fo5j4KcTVutZJ5KWRRJrbxzmxA4td8NfEdrPh7/go-libp2p-routing"
	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	isd "gx/ipfs/QmZmmuAXgX73UQmX1jRKjTGmjzq24Jinqkq8vzkBtno4uX/go-is-domain"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

// mpns (a multi-protocol NameSystem) implements generic IPFS naming.
//
// Uses several Resolvers:
// (a) IPFS routing naming: SFS-like PKI names.
// (b) dns domains: resolves using links in DNS TXT records
// (c) proquints: interprets string as the raw byte data.
//
// It can only publish to: (a) IPFS routing naming.
//
type mpns struct {
	resolvers  map[string]resolver
	publishers map[string]Publisher
}

// NewNameSystem will construct the IPFS naming system based on Routing
func NewNameSystem(r routing.ValueStore, ds ds.Datastore, cachesize int, dnsResolver Resolver) NameSystem {
	m := &mpns{
		resolvers: map[string]resolver{
			"proquint": new(ProquintResolver),
			"dht":      NewRoutingResolver(r, cachesize, ds),
		},
		publishers: map[string]Publisher{
			"dht": NewRoutingPublisher(r, ds),
		},
	}
	if dnsResolver != nil {
		m.resolvers["dns"] = dnsResolver.(*DNSResolver)
	}
	return m
}

// AddPubsubNameSystem adds the pubsub publisher and resolver to the namesystem
func AddPubsubNameSystem(ctx context.Context, ns NameSystem, host p2phost.Host, r routing.IpfsRouting, ds ds.Datastore, ps *floodsub.PubSub) error {
	mpns, ok := ns.(*mpns)
	if !ok {
		return errors.New("unexpected NameSystem; not an mpns instance")
	}

	pkf, ok := r.(routing.PubKeyFetcher)
	if !ok {
		return errors.New("unexpected IpfsRouting; not a PubKeyFetcher instance")
	}

	mpns.resolvers["pubsub"] = NewPubsubResolver(ctx, host, ds, r, pkf, ps)
	mpns.publishers["pubsub"] = NewPubsubPublisher(ctx, host, ds, r, ps)
	return nil
}

const DefaultResolverCacheTTL = time.Minute

// Resolve implements Resolver.
func (ns *mpns) Resolve(ctx context.Context, name string, options ...opts.ResolveOpt) (path.Path, error) {
	if strings.HasPrefix(name, "/ipfs/") {
		return path.ParsePath(name)
	}

	if !strings.HasPrefix(name, "/") {
		return path.ParsePath("/ipfs/" + name)
	}

	return resolve(ctx, ns, name, opts.ProcessOpts(options), "/ipns/")
}

// resolveOnce implements resolver.
func (ns *mpns) resolveOnce(ctx context.Context, name string, options *opts.ResolveOpts) (path.Path, error) {
	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}
	segments := strings.SplitN(name, "/", 4)
	if len(segments) < 3 || segments[0] != "" {
		log.Debugf("invalid name syntax for %s", name)
		return "", ErrResolveFailed
	}

	makePath := func(p path.Path) (path.Path, error) {
		if len(segments) > 3 {
			return path.FromSegments("", strings.TrimRight(p.String(), "/"), segments[3])
		} else {
			return p, nil
		}
	}

	// Resolver selection:
	// 1. if it is a multihash resolve through "pubsub" (if available),
	//    with fallback to "dht"
	// 2. if it is a domain name, resolve through "dns"
	// 3. otherwise resolve through the "proquint" resolver
	key := segments[2]
	fullKey := key
	if strings.Contains(key, ":") {
		s := strings.Split(key, ":")
		key = s[0]
	}

	_, err := mh.FromB58String(key)
	if err == nil {
		res, ok := ns.resolvers["pubsub"]
		if ok {
			p, err := res.resolveOnce(ctx, fullKey, options)
			if err == nil {
				return makePath(p)
			}
		}

		res, ok = ns.resolvers["dht"]
		if ok {
			p, err := res.resolveOnce(ctx, fullKey, options)
			if err == nil {
				return makePath(p)
			}
		}

		// Try pubsub again. The reason for this is pubsub will return an error if it's not subscribed
		// even though a record might exist in cache. The reason is to force a DHT query to update the
		// cache. However, the first pubsub resolve request will subscribe to the name and subsequent
		// resolves will return from cache. So the pattern is.. if not subscribed, try DHT first, if that
		// fails return from cache.
		res, ok = ns.resolvers["pubsub"]
		if ok {
			options.DoNotSubscribe = true
			p, err := res.resolveOnce(ctx, fullKey, options)
			if err == nil {
				return makePath(p)
			}
		}

		return "", ErrResolveFailed
	}

	if isd.IsDomain(key) {
		res, ok := ns.resolvers["dns"]
		if ok {
			p, err := res.resolveOnce(ctx, key, options)
			if err == nil {
				return makePath(p)
			}
		}

		return "", ErrResolveFailed
	}

	res, ok := ns.resolvers["proquint"]
	if ok {
		p, err := res.resolveOnce(ctx, key, options)
		if err == nil {
			return makePath(p)
		}

		return "", ErrResolveFailed
	}

	log.Debugf("no resolver found for %s", name)
	return "", ErrResolveFailed
}

// Publish implements Publisher
func (ns *mpns) Publish(ctx context.Context, name ci.PrivKey, value path.Path) error {
	return ns.PublishWithEOL(ctx, name, value, time.Now().Add(DefaultRecordTTL))
}

func (ns *mpns) PublishWithEOL(ctx context.Context, name ci.PrivKey, value path.Path, eol time.Time) error {
	var dhtErr error

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		dhtErr = ns.publishers["dht"].PublishWithEOL(ctx, name, value, eol)
		if dhtErr == nil {
			ns.addToDHTCache(name, value, eol)
		}
		wg.Done()
	}()

	pub, ok := ns.publishers["pubsub"]
	if ok {
		wg.Add(1)
		go func() {
			err := pub.PublishWithEOL(ctx, name, value, eol)
			if err != nil {
				log.Warningf("error publishing %s with pubsub: %s", name, err.Error())
			}
			wg.Done()
		}()
	}

	wg.Wait()
	return dhtErr
}

func (ns *mpns) addToDHTCache(key ci.PrivKey, value path.Path, eol time.Time) {
	rr, ok := ns.resolvers["dht"].(*routingResolver)
	if !ok {
		// should never happen, purely for sanity
		log.Panicf("unexpected type %T as DHT resolver.", ns.resolvers["dht"])
	}
	if rr.cache == nil {
		// resolver has no caching
		return
	}

	var err error
	value, err = path.ParsePath(value.String())
	if err != nil {
		log.Error("could not parse path")
		return
	}

	name, err := peer.IDFromPrivateKey(key)
	if err != nil {
		log.Error("while adding to cache, could not get peerid from private key")
		return
	}

	if time.Now().Add(DefaultResolverCacheTTL).Before(eol) {
		eol = time.Now().Add(DefaultResolverCacheTTL)
	}
	rr.cache.Add(name.Pretty(), cacheEntry{
		val: value,
		eol: eol,
	})
}

// GetResolver implements ResolverLookup
func (ns *mpns) GetResolver(subs string) (Resolver, bool) {
	res, ok := ns.resolvers[subs]
	if ok {
		ires, ok := res.(Resolver)
		if ok {
			return ires, true
		}
	}

	return nil, false
}
