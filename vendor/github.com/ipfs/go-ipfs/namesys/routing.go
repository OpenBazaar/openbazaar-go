package namesys

import (
	"context"
	"strings"
	"time"

	opts "github.com/ipfs/go-ipfs/namesys/opts"
	pb "github.com/ipfs/go-ipfs/namesys/pb"
	path "github.com/ipfs/go-ipfs/path"

	u "gx/ipfs/QmNiJuT8Ja3hMVpBHXv3Q6dwmperaQ6JjLtpMQgMCD7xvx/go-ipfs-util"
	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	routing "gx/ipfs/QmTiWLZ6Fo5j4KcTVutZJ5KWRRJrbxzmxA4td8NfEdrPh7/go-libp2p-routing"
	lru "gx/ipfs/QmVYxfoJQiZijTgPNHCHgHELvQpbsJNTg6Crmc3dQkj3yy/golang-lru"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmTmqJGRQfuH8eKWD1FjThwPRipt1QhqJQNZ8MpzmfAAxo/go-ipfs-ds-help"
)

var log = logging.Logger("namesys")
var keyCachePrefix= "IPNSPUBKEYCACHE_"

// routingResolver implements NSResolver for the main IPFS SFS-like naming
type routingResolver struct {
	routing            routing.ValueStore
	datastore          ds.Datastore
	cache              *lru.Cache
}

func (r *routingResolver) cacheGet(name string) (path.Path, bool) {
	if r.cache == nil {
		return "", false
	}

	ientry, ok := r.cache.Get(name)
	if !ok {
		return "", false
	}

	entry, ok := ientry.(cacheEntry)
	if !ok {
		// should never happen, purely for sanity
		log.Panicf("unexpected type %T in cache for %q.", ientry, name)
	}

	if time.Now().Before(entry.eol) {
		return entry.val, true
	}

	r.cache.Remove(name)

	return "", false
}

func (r *routingResolver) cacheSet(name string, val path.Path, rec *pb.IpnsEntry) {
	if r.cache == nil {
		return
	}

	// if completely unspecified, just use one minute
	ttl := DefaultResolverCacheTTL
	if rec.Ttl != nil {
		recttl := time.Duration(rec.GetTtl())
		if recttl >= 0 {
			ttl = recttl
		}
	}

	cacheTil := time.Now().Add(ttl)
	eol, ok := checkEOL(rec)
	if ok && eol.Before(cacheTil) {
		cacheTil = eol
	}

	r.cache.Add(name, cacheEntry{
		val: val,
		eol: cacheTil,
	})
}

type cacheEntry struct {
	val path.Path
	eol time.Time
}

// NewRoutingResolver constructs a name resolver using the IPFS Routing system
// to implement SFS-like naming on top.
// cachesize is the limit of the number of entries in the lru cache. Setting it
// to '0' will disable caching.
func NewRoutingResolver(route routing.ValueStore, cachesize int, ds ds.Datastore) *routingResolver {
	if route == nil {
		panic("attempt to create resolver with nil routing system")
	}

	var cache *lru.Cache
	if cachesize > 0 {
		cache, _ = lru.New(cachesize)
	}

	return &routingResolver{
		routing:            route,
		cache:              cache,
		datastore:          ds,
	}
}

// Resolve implements Resolver.
func (r *routingResolver) Resolve(ctx context.Context, name string, options ...opts.ResolveOpt) (path.Path, error) {
	return resolve(ctx, r, name, opts.ProcessOpts(options), "/ipns/")
}

// resolveOnce implements resolver. Uses the IPFS routing system to
// resolve SFS-like names.
func (r *routingResolver) resolveOnce(ctx context.Context, pname string, options *opts.ResolveOpts) (path.Path, error) {
	log.Debugf("RoutingResolver resolving %s", pname)
	cached, ok := r.cacheGet(pname)
	if ok {
		return cached, nil
	}

	if options.DhtTimeout != 0 {
		// Resolution must complete within the timeout
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.DhtTimeout)
		defer cancel()
	}

	name := strings.TrimPrefix(pname, "/ipns/")
	split := strings.SplitN(name, ":", 2)
	name = split[0]

	hash, err := mh.FromB58String(name)
	if err != nil {
		// name should be a multihash. if it isn't, error out here.
		log.Debugf("RoutingResolver: bad input hash: [%s]\n", name)
		return "", err
	}

	suffix := ""
	if len(split) > 1 {
		suffix = ":" + split[1]
		name += suffix
	}

	// use the routing system to get the name.
	// /ipns/<name>
	h := []byte("/ipns/" + string(hash) + suffix)

	var entry *pb.IpnsEntry
	var pubkey ci.PubKey
	var val []byte

	resp := make(chan error, 2)
	go func() {
		ipnsKey := string(h)
		val, err = r.routing.GetValue(ctx, ipnsKey)
		if err != nil {
			log.Warning("RoutingResolve get failed.")
			resp <- err
			return
		}

		entry = new(pb.IpnsEntry)
		err = proto.Unmarshal(val, entry)
		if err != nil {
			resp <- err
			return
		}
		resp <- nil
	}()

	go func() {
		val, err := r.datastore.Get(ds.NewKey(keyCachePrefix + pname))
		if err == nil {
			b, ok := val.([]byte)
			if ok {
				pubkey, err = ci.UnmarshalPublicKey(b)
				if err == nil {
					resp <- nil
					return
				}
			}
		}
		// name should be a public key retrievable from ipfs
		pubk, err := routing.GetPublicKey(r.routing, ctx, hash)
		if err != nil {
			resp <- err
			return
		}

		pubkey = pubk
		resp <- nil
	}()

	for i := 0; i < 2; i++ {
		err = <-resp
		if err != nil {
			return "", err
		}
	}

	pubkeyBytes, _ := pubkey.Bytes()

	// check for old style record:
	valh, err := mh.Cast(entry.GetValue())
	if err != nil {
		// Not a multihash, probably a new record
		p, err := path.ParsePath(string(entry.GetValue()))
		if err != nil {
			return "", err
		}

		r.cacheSet(pname, p, entry)
		go func() {
			putToDatabase(r.datastore, pname, val, pubkeyBytes)
		}()
		return p, nil
	} else {
		// Its an old style multihash record
		log.Debugf("encountered CIDv0 ipns entry: %s", valh)
		p := path.FromCid(cid.NewCidV0(valh))
		r.cacheSet(pname, p, entry)
		go func() {
			putToDatabase(r.datastore, pname, val, pubkeyBytes)
		}()
		return p, nil
	}
}

func (r *routingResolver) getValue(ctx context.Context, ipnsKey string, options *opts.ResolveOpts) ([]byte, error) {
	// Get specified number of values from the DHT
	vals, err := r.routing.GetValues(ctx, ipnsKey, int(options.DhtRecordCount))
	if err != nil {
		return nil, err
	}

	// Select the best value
	recs := make([][]byte, 0, len(vals))
	for _, v := range vals {
		if v.Val != nil {
			recs = append(recs, v.Val)
		}
	}

	if len(recs) == 0 {
		return nil, routing.ErrNotFound
	}

	i, err := IpnsSelectorFunc(ipnsKey, recs)
	if err != nil {
		return nil, err
	}

	best := recs[i]
	if best == nil {
		log.Errorf("GetValues %s yielded record with nil value", ipnsKey)
		return nil, routing.ErrNotFound
	}

	return best, nil
}

func checkEOL(e *pb.IpnsEntry) (time.Time, bool) {
	if e.GetValidityType() == pb.IpnsEntry_EOL {
		eol, err := u.ParseRFC3339(string(e.GetValidity()))
		if err != nil {
			return time.Time{}, false
		}
		return eol, true
	}
	return time.Time{}, false
}

func putToDatabase(datastore ds.Datastore, name string, ipnsRec, pubkey []byte) {
	datastore.Put(dshelp.NewKeyFromBinary([]byte(name)), ipnsRec)
	datastore.Put(ds.NewKey(keyCachePrefix+name), pubkey)
}