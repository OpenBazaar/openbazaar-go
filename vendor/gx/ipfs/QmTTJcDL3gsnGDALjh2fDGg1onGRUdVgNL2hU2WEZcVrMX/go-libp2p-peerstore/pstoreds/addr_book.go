package pstoreds

import (
	"context"
	"encoding/binary"
	"errors"
	"time"

	lru "gx/ipfs/QmQjMHF8ptRgx4E57UFMiT4YM6kqaJeYxZ1MCDX23aw4rK/golang-lru"
	base32 "gx/ipfs/QmfVj3x4D6Jkq9SEoi5n2NmoUomLwoeiwnYz2KQa15wRw6/base32"

	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	query "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/query"

	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"

	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
	pstoremem "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore/pstoremem"
)

var (
	log = logging.Logger("peerstore/ds")
	// The maximum representable value in time.Time is time.Unix(1<<63-62135596801, 999999999).
	// But it's too brittle and implementation-dependent, so we prefer to use 1<<62, which is in the
	// year 146138514283. We're safe.
	maxTime = time.Unix(1<<62, 0)

	ErrTTLDatastore = errors.New("datastore must provide TTL support")
)

// Peer addresses are stored under the following db key pattern:
// /peers/addr/<b32 peer id no padding>/<hash of maddr>
var abBase = ds.NewKey("/peers/addrs")

var _ pstore.AddrBook = (*dsAddrBook)(nil)

// dsAddrBook is an address book backed by a Datastore with both an
// in-memory TTL manager and an in-memory address stream manager.
type dsAddrBook struct {
	cache        cache
	ds           ds.TxnDatastore
	subsManager  *pstoremem.AddrSubManager
	writeRetries int
}

type ttlWriteMode int

const (
	ttlOverride ttlWriteMode = iota
	ttlExtend
)

type cacheEntry struct {
	expiration time.Time
	addrs      []ma.Multiaddr
}

type addrRecord struct {
	ttl  time.Duration
	addr ma.Multiaddr
}

func (ar *addrRecord) MarshalBinary() ([]byte, error) {
	ttlB := make([]byte, 8)
	binary.LittleEndian.PutUint64(ttlB, uint64(ar.ttl))
	return append(ttlB, ar.addr.Bytes()...), nil
}

func (ar *addrRecord) UnmarshalBinary(b []byte) error {
	ar.ttl = time.Duration(binary.LittleEndian.Uint64(b))
	// this had been serialized by us, no need to check for errors
	ar.addr, _ = ma.NewMultiaddrBytes(b[8:])
	return nil
}

// NewAddrBook initializes a new address book given a
// Datastore instance, a context for managing the TTL manager,
// and the interval at which the TTL manager should sweep the Datastore.
func NewAddrBook(ctx context.Context, store ds.TxnDatastore, opts Options) (*dsAddrBook, error) {
	if _, ok := store.(ds.TTLDatastore); !ok {
		return nil, ErrTTLDatastore
	}

	var (
		cache cache = &noopCache{}
		err   error
	)

	if opts.CacheSize > 0 {
		if cache, err = lru.NewARC(int(opts.CacheSize)); err != nil {
			return nil, err
		}
	}

	mgr := &dsAddrBook{
		cache:        cache,
		ds:           store,
		subsManager:  pstoremem.NewAddrSubManager(),
		writeRetries: int(opts.WriteRetries),
	}
	return mgr, nil
}

func keysAndAddrs(p peer.ID, addrs []ma.Multiaddr) ([]ds.Key, []ma.Multiaddr, error) {
	var (
		keys      = make([]ds.Key, len(addrs))
		clean     = make([]ma.Multiaddr, len(addrs))
		parentKey = abBase.ChildString(base32.RawStdEncoding.EncodeToString([]byte(p)))
		i         = 0
	)

	for _, addr := range addrs {
		if addr == nil {
			continue
		}

		hash, err := mh.Sum((addr).Bytes(), mh.MURMUR3, -1)
		if err != nil {
			return nil, nil, err
		}
		keys[i] = parentKey.ChildString(base32.RawStdEncoding.EncodeToString(hash))
		clean[i] = addr
		i++
	}

	return keys[:i], clean[:i], nil
}

// AddAddr will add a new address if it's not already in the AddrBook.
func (mgr *dsAddrBook) AddAddr(p peer.ID, addr ma.Multiaddr, ttl time.Duration) {
	mgr.AddAddrs(p, []ma.Multiaddr{addr}, ttl)
}

// AddAddrs will add many new addresses if they're not already in the AddrBook.
func (mgr *dsAddrBook) AddAddrs(p peer.ID, addrs []ma.Multiaddr, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	mgr.setAddrs(p, addrs, ttl, ttlExtend)
}

// SetAddr will add or update the TTL of an address in the AddrBook.
func (mgr *dsAddrBook) SetAddr(p peer.ID, addr ma.Multiaddr, ttl time.Duration) {
	addrs := []ma.Multiaddr{addr}
	mgr.SetAddrs(p, addrs, ttl)
}

// SetAddrs will add or update the TTLs of addresses in the AddrBook.
func (mgr *dsAddrBook) SetAddrs(p peer.ID, addrs []ma.Multiaddr, ttl time.Duration) {
	if ttl <= 0 {
		mgr.deleteAddrs(p, addrs)
		return
	}
	mgr.setAddrs(p, addrs, ttl, ttlOverride)
}

func (mgr *dsAddrBook) deleteAddrs(p peer.ID, addrs []ma.Multiaddr) error {
	// Keys and cleaned up addresses.
	keys, addrs, err := keysAndAddrs(p, addrs)
	if err != nil {
		return err
	}

	mgr.cache.Remove(p)
	// Attempt transactional KV deletion.
	for i := 0; i < mgr.writeRetries; i++ {
		if err = mgr.dbDelete(keys); err == nil {
			break
		}
		log.Errorf("failed to delete addresses for peer %s: %s\n", p.Pretty(), err)
	}

	if err != nil {
		log.Errorf("failed to avoid write conflict for peer %s after %d retries: %v\n", p.Pretty(), mgr.writeRetries, err)
		return err
	}

	return nil
}

func (mgr *dsAddrBook) setAddrs(p peer.ID, addrs []ma.Multiaddr, ttl time.Duration, mode ttlWriteMode) error {
	// Keys and cleaned up addresses.
	keys, addrs, err := keysAndAddrs(p, addrs)
	if err != nil {
		return err
	}

	mgr.cache.Remove(p)
	// Attempt transactional KV insertion.
	var existed []bool
	for i := 0; i < mgr.writeRetries; i++ {
		if existed, err = mgr.dbInsert(keys, addrs, ttl, mode); err == nil {
			break
		}
		log.Errorf("failed to write addresses for peer %s: %s\n", p.Pretty(), err)
	}

	if err != nil {
		log.Errorf("failed to avoid write conflict for peer %s after %d retries: %v\n", p.Pretty(), mgr.writeRetries, err)
		return err
	}

	// Update was successful, so broadcast event only for new addresses.
	for i, _ := range keys {
		if !existed[i] {
			mgr.subsManager.BroadcastAddr(p, addrs[i])
		}
	}
	return nil
}

// dbInsert performs a transactional insert of the provided keys and values.
func (mgr *dsAddrBook) dbInsert(keys []ds.Key, addrs []ma.Multiaddr, ttl time.Duration, mode ttlWriteMode) ([]bool, error) {
	var (
		err     error
		existed = make([]bool, len(keys))
		exp     = time.Now().Add(ttl)
	)

	txn, err := mgr.ds.NewTransaction(false)
	if err != nil {
		return nil, err
	}
	defer txn.Discard()

	ttltxn := txn.(ds.TTLDatastore)
	for i, key := range keys {
		// Check if the key existed previously.
		if existed[i], err = ttltxn.Has(key); err != nil {
			log.Errorf("transaction failed and aborted while checking key existence: %s, cause: %v", key.String(), err)
			return nil, err
		}

		// The key embeds a hash of the value, so if it existed, we can safely skip the insert and
		// just update the TTL.
		if existed[i] {
			switch mode {
			case ttlOverride:
				err = ttltxn.SetTTL(key, ttl)
			case ttlExtend:
				var curr time.Time
				if curr, err = ttltxn.GetExpiration(key); err == nil && exp.After(curr) {
					err = ttltxn.SetTTL(key, ttl)
				}
			}
			if err != nil {
				// mode will be printed as an int
				log.Errorf("failed while updating the ttl for key: %s, mode: %v, cause: %v", key.String(), mode, err)
				return nil, err
			}
			continue
		}

		r := &addrRecord{
			ttl:  ttl,
			addr: addrs[i],
		}
		value, _ := r.MarshalBinary()
		if err = ttltxn.PutWithTTL(key, value, ttl); err != nil {
			log.Errorf("transaction failed and aborted while setting key: %s, cause: %v", key.String(), err)
			return nil, err
		}
	}

	if err = txn.Commit(); err != nil {
		log.Errorf("failed to commit transaction when setting keys, cause: %v", err)
		return nil, err
	}

	return existed, nil
}

// UpdateAddrs will update any addresses for a given peer and TTL combination to
// have a new TTL.
func (mgr *dsAddrBook) UpdateAddrs(p peer.ID, oldTTL time.Duration, newTTL time.Duration) {
	mgr.cache.Remove(p)

	var err error
	for i := 0; i < mgr.writeRetries; i++ {
		if err = mgr.dbUpdateTTL(p, oldTTL, newTTL); err == nil {
			break
		}
		log.Errorf("failed to update ttlsfor peer %s: %s\n", p.Pretty(), err)
	}

	if err != nil {
		log.Errorf("failed to avoid write conflict when updating ttls for peer %s after %d retries: %v\n",
			p.Pretty(), mgr.writeRetries, err)
	}
}

func (mgr *dsAddrBook) dbUpdateTTL(p peer.ID, oldTTL time.Duration, newTTL time.Duration) error {
	var (
		prefix  = abBase.ChildString(base32.RawStdEncoding.EncodeToString([]byte(p)))
		q       = query.Query{Prefix: prefix.String(), KeysOnly: false}
		results query.Results
		err     error
	)

	txn, err := mgr.ds.NewTransaction(false)
	if err != nil {
		return err
	}
	defer txn.Discard()

	if results, err = txn.Query(q); err != nil {
		return err
	}
	defer results.Close()

	ttltxn := txn.(ds.TTLDatastore)
	r := &addrRecord{}
	for result := range results.Next() {
		r.UnmarshalBinary(result.Value)
		if r.ttl != oldTTL {
			continue
		}

		r.ttl = newTTL
		value, _ := r.MarshalBinary()
		if err = ttltxn.PutWithTTL(ds.RawKey(result.Key), value, newTTL); err != nil {
			return err
		}
	}

	if err := txn.Commit(); err != nil {
		log.Errorf("failed to commit transaction when updating ttls, cause: %v", err)
		return err
	}

	return nil
}

// Addrs returns all of the non-expired addresses for a given peer.
func (mgr *dsAddrBook) Addrs(p peer.ID) []ma.Multiaddr {
	var (
		prefix  = abBase.ChildString(base32.RawStdEncoding.EncodeToString([]byte(p)))
		q       = query.Query{Prefix: prefix.String(), KeysOnly: false, ReturnExpirations: true}
		results query.Results
		err     error
	)

	// Check the cache and return the entry only if it hasn't expired; if expired, remove.
	if e, ok := mgr.cache.Get(p); ok {
		entry := e.(cacheEntry)
		if entry.expiration.After(time.Now()) {
			addrs := make([]ma.Multiaddr, len(entry.addrs))
			copy(addrs, entry.addrs)
			return addrs
		} else {
			mgr.cache.Remove(p)
		}
	}

	txn, err := mgr.ds.NewTransaction(true)
	if err != nil {
		return nil
	}
	defer txn.Discard()

	if results, err = txn.Query(q); err != nil {
		log.Error(err)
		return nil
	}
	defer results.Close()

	var addrs []ma.Multiaddr
	var r addrRecord
	// used to set the expiration for the entire cache entry
	earliestExp := maxTime
	for result := range results.Next() {
		if err = r.UnmarshalBinary(result.Value); err == nil {
			addrs = append(addrs, r.addr)
		}

		if exp := result.Expiration; !exp.IsZero() && exp.Before(earliestExp) {
			earliestExp = exp
		}
	}

	// Store a copy in the cache.
	addrsCpy := make([]ma.Multiaddr, len(addrs))
	copy(addrsCpy, addrs)
	entry := cacheEntry{addrs: addrsCpy, expiration: earliestExp}
	mgr.cache.Add(p, entry)

	return addrs
}

// Peers returns all of the peer IDs for which the AddrBook has addresses.
func (mgr *dsAddrBook) PeersWithAddrs() peer.IDSlice {
	ids, err := uniquePeerIds(mgr.ds, abBase, func(result query.Result) string {
		return ds.RawKey(result.Key).Parent().Name()
	})
	if err != nil {
		log.Errorf("error while retrieving peers with addresses: %v", err)
	}
	return ids
}

// AddrStream returns a channel on which all new addresses discovered for a
// given peer ID will be published.
func (mgr *dsAddrBook) AddrStream(ctx context.Context, p peer.ID) <-chan ma.Multiaddr {
	initial := mgr.Addrs(p)
	return mgr.subsManager.AddrStream(ctx, p, initial)
}

// ClearAddrs will delete all known addresses for a peer ID.
func (mgr *dsAddrBook) ClearAddrs(p peer.ID) {
	var (
		err      error
		prefix   = abBase.ChildString(base32.RawStdEncoding.EncodeToString([]byte(p)))
		deleteFn func() error
	)

	if e, ok := mgr.cache.Peek(p); ok {
		mgr.cache.Remove(p)
		keys, _, _ := keysAndAddrs(p, e.(cacheEntry).addrs)
		deleteFn = func() error {
			return mgr.dbDelete(keys)
		}
	} else {
		deleteFn = func() error {
			return mgr.dbDeleteIter(prefix)
		}
	}

	// Attempt transactional KV deletion.
	for i := 0; i < mgr.writeRetries; i++ {
		if err = deleteFn(); err == nil {
			break
		}
		log.Errorf("failed to clear addresses for peer %s: %s\n", p.Pretty(), err)
	}

	if err != nil {
		log.Errorf("failed to clear addresses for peer %s after %d attempts\n", p.Pretty(), mgr.writeRetries)
	}
}

// dbDelete transactionally deletes the provided keys.
func (mgr *dsAddrBook) dbDelete(keys []ds.Key) error {
	var err error

	txn, err := mgr.ds.NewTransaction(false)
	if err != nil {
		return err
	}
	defer txn.Discard()

	for _, key := range keys {
		if err = txn.Delete(key); err != nil {
			log.Errorf("failed to delete key: %s, cause: %v", key.String(), err)
			return err
		}
	}

	if err = txn.Commit(); err != nil {
		log.Errorf("failed to commit transaction when deleting keys, cause: %v", err)
		return err
	}

	return nil
}

// dbDeleteIter removes all entries whose keys are prefixed with the argument.
// it returns a slice of the removed keys in case it's needed
func (mgr *dsAddrBook) dbDeleteIter(prefix ds.Key) error {
	q := query.Query{Prefix: prefix.String(), KeysOnly: true}

	txn, err := mgr.ds.NewTransaction(false)
	if err != nil {
		return err
	}
	defer txn.Discard()

	results, err := txn.Query(q)
	if err != nil {
		log.Errorf("failed to fetch all keys prefixed with: %s, cause: %v", prefix.String(), err)
		return err
	}

	var keys = make([]ds.Key, 0, 4) // cap: 4 to reduce allocs
	var key ds.Key
	for result := range results.Next() {
		key = ds.RawKey(result.Key)
		keys = append(keys, key)

		if err = txn.Delete(key); err != nil {
			log.Errorf("failed to delete key: %s, cause: %v", key.String(), err)
			return err
		}
	}

	if err = results.Close(); err != nil {
		log.Errorf("failed to close cursor, cause: %v", err)
		return err
	}

	if err = txn.Commit(); err != nil {
		log.Errorf("failed to commit transaction when deleting keys, cause: %v", err)
		return err
	}

	return nil
}
