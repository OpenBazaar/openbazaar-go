package namesys

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	dshelp "gx/ipfs/QmS73grfbWgWrNztd8Lns9GCG3jjRNDfcPYg2VYQzKDZSt/go-ipfs-ds-help"
	pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
	pubsub "gx/ipfs/QmY4dowpPFCBsbaoaJc9mNWso64eDJsm32LJznwPNaAiJG/go-libp2p-pubsub"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
	record "gx/ipfs/Qma9Eqp16mNHDX1EL73pcxhFfzbyXVcAYtaDd1xdmDRDtL/go-libp2p-record"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	dssync "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/sync"
	routing "gx/ipfs/QmcQ81jSyWCp1jpkQ8CMbtpXT3jK7Wg6ZtYmoyWFgBoF9c/go-libp2p-routing"
	ropts "gx/ipfs/QmcQ81jSyWCp1jpkQ8CMbtpXT3jK7Wg6ZtYmoyWFgBoF9c/go-libp2p-routing/options"
	p2phost "gx/ipfs/QmdJfsSbKSZnMkfZ1kpopiyB9i3Hd6cp8VKWZmtWPa7Moc/go-libp2p-host"
)

var log = logging.Logger("pubsub-valuestore")

type watchGroup struct {
	// Note: this chan must be buffered, see notifyWatchers
	listeners map[chan []byte]struct{}
}

type PubsubValueStore struct {
	ctx  context.Context
	ds   ds.Datastore
	host p2phost.Host
	cr   routing.ContentRouting
	ps   *pubsub.PubSub

	// Map of keys to subscriptions.
	//
	// If a key is present but the subscription is nil, we've bootstrapped
	// but haven't subscribed.
	mx   sync.Mutex
	subs map[string]*pubsub.Subscription

	watchLk  sync.Mutex
	watching map[string]*watchGroup

	Validator record.Validator
}

// NewPubsubPublisher constructs a new Publisher that publishes IPNS records through pubsub.
// The constructor interface is complicated by the need to bootstrap the pubsub topic.
// This could be greatly simplified if the pubsub implementation handled bootstrap itself
func NewPubsubValueStore(ctx context.Context, host p2phost.Host, cr routing.ContentRouting, ps *pubsub.PubSub, validator record.Validator) *PubsubValueStore {
	return &PubsubValueStore{
		ctx: ctx,

		ds:   dssync.MutexWrap(ds.NewMapDatastore()),
		host: host, // needed for pubsub bootstrap
		cr:   cr,   // needed for pubsub bootstrap
		ps:   ps,

		subs:     make(map[string]*pubsub.Subscription),
		watching: make(map[string]*watchGroup),

		Validator: validator,
	}
}

// Publish publishes an IPNS record through pubsub with default TTL
func (p *PubsubValueStore) PutValue(ctx context.Context, key string, value []byte, opts ...ropts.Option) error {
	p.mx.Lock()
	_, bootstraped := p.subs[key]

	if !bootstraped {
		p.subs[key] = nil
		p.mx.Unlock()

		bootstrapPubsub(p.ctx, p.cr, p.host, key)
	} else {
		p.mx.Unlock()
	}

	log.Debugf("PubsubPublish: publish value for key", key)
	return p.ps.Publish(key, value)
}

func (p *PubsubValueStore) isBetter(key string, val []byte) bool {
	if p.Validator.Validate(key, val) != nil {
		return false
	}

	old, err := p.getLocal(key)
	if err != nil {
		// If the old one is invalid, the new one is *always* better.
		return true
	}

	// Same record. Possible DoS vector, should consider failing?
	if bytes.Equal(old, val) {
		return true
	}

	i, err := p.Validator.Select(key, [][]byte{val, old})
	return err == nil && i == 0
}

func (p *PubsubValueStore) Subscribe(key string) error {
	p.mx.Lock()
	// see if we already have a pubsub subscription; if not, subscribe
	sub := p.subs[key]
	p.mx.Unlock()

	if sub != nil {
		return nil
	}

	// Ignore the error. We have to check again anyways to make sure the
	// record hasn't expired.
	//
	// Also, make sure to do this *before* subscribing.
	p.ps.RegisterTopicValidator(key, func(ctx context.Context, msg *pubsub.Message) bool {
		return p.isBetter(key, msg.GetData())
	})

	sub, err := p.ps.Subscribe(key)
	if err != nil {
		p.mx.Unlock()
		return err
	}

	p.mx.Lock()
	existingSub, bootstraped := p.subs[key]
	if existingSub != nil {
		p.mx.Unlock()
		sub.Cancel()
		return nil
	}

	p.subs[key] = sub
	ctx, cancel := context.WithCancel(p.ctx)
	go p.handleSubscription(sub, key, cancel)
	p.mx.Unlock()

	log.Debugf("PubsubResolve: subscribed to %s", key)

	if !bootstraped {
		// TODO: Deal with publish then resolve case? Cancel behaviour changes.
		go bootstrapPubsub(ctx, p.cr, p.host, key)
	}
	return nil
}

func (p *PubsubValueStore) getLocal(key string) ([]byte, error) {
	val, err := p.ds.Get(dshelp.NewKeyFromBinary([]byte(key)))
	if err != nil {
		// Don't invalidate due to ds errors.
		if err == ds.ErrNotFound {
			err = routing.ErrNotFound
		}
		return nil, err
	}

	// If the old one is invalid, the new one is *always* better.
	if err := p.Validator.Validate(key, val); err != nil {
		return nil, err
	}
	return val, nil
}

func (p *PubsubValueStore) GetValue(ctx context.Context, key string, opts ...ropts.Option) ([]byte, error) {
	if err := p.Subscribe(key); err != nil {
		return nil, err
	}

	return p.getLocal(key)
}

func (p *PubsubValueStore) SearchValue(ctx context.Context, key string, opts ...ropts.Option) (<-chan []byte, error) {
	if err := p.Subscribe(key); err != nil {
		return nil, err
	}

	p.watchLk.Lock()
	defer p.watchLk.Unlock()

	out := make(chan []byte, 1)
	lv, err := p.getLocal(key)
	if err == nil {
		out <- lv
		close(out)
		return out, nil
	}

	wg, ok := p.watching[key]
	if !ok {
		wg = &watchGroup{
			listeners: map[chan []byte]struct{}{},
		}
		p.watching[key] = wg
	}

	proxy := make(chan []byte, 1)

	ctx, cancel := context.WithCancel(ctx)
	wg.listeners[proxy] = struct{}{}

	go func() {
		defer func() {
			cancel()

			p.watchLk.Lock()
			delete(wg.listeners, proxy)

			if _, ok := p.watching[key]; len(wg.listeners) == 0 && ok {
				delete(p.watching, key)
			}
			p.watchLk.Unlock()

			close(out)
		}()

		for {
			select {
			case val, ok := <-proxy:
				if !ok {
					return
				}

				// outCh is buffered, so we just put the value or swap it for the newer one
				select {
				case out <- val:
				case <-out:
					out <- val
				}

				// 1 is good enough
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// GetSubscriptions retrieves a list of active topic subscriptions
func (p *PubsubValueStore) GetSubscriptions() []string {
	p.mx.Lock()
	defer p.mx.Unlock()

	var res []string
	for sub := range p.subs {
		res = append(res, sub)
	}

	return res
}

// Cancel cancels a topic subscription; returns true if an active
// subscription was canceled
func (p *PubsubValueStore) Cancel(name string) (bool, error) {
	p.mx.Lock()
	defer p.mx.Unlock()

	p.watchLk.Lock()
	if _, wok := p.watching[name]; wok {
		p.watchLk.Unlock()
		return false, errors.New("key has active subscriptions")
	}
	p.watchLk.Unlock()

	sub, ok := p.subs[name]
	if ok {
		sub.Cancel()
		delete(p.subs, name)
	}

	return ok, nil
}

func (p *PubsubValueStore) handleSubscription(sub *pubsub.Subscription, key string, cancel func()) {
	defer sub.Cancel()
	defer cancel()

	for {
		msg, err := sub.Next(p.ctx)
		if err != nil {
			if err != context.Canceled {
				log.Warningf("PubsubResolve: subscription error in %s: %s", key, err.Error())
			}
			return
		}
		if p.isBetter(key, msg.GetData()) {
			err := p.ds.Put(dshelp.NewKeyFromBinary([]byte(key)), msg.GetData())
			if err != nil {
				log.Warningf("PubsubResolve: error writing update for %s: %s", key, err)
			}
			p.notifyWatchers(key, msg.GetData())
		}
	}
}

func (p *PubsubValueStore) notifyWatchers(key string, data []byte) {
	p.watchLk.Lock()
	defer p.watchLk.Unlock()
	sg, ok := p.watching[key]
	if !ok {
		return
	}

	for watcher := range sg.listeners {
		select {
		case <-watcher:
			watcher <- data
		case watcher <- data:
		}
	}
}

// rendezvous with peers in the name topic through provider records
// Note: rendezvous/boostrap should really be handled by the pubsub implementation itself!
func bootstrapPubsub(ctx context.Context, cr routing.ContentRouting, host p2phost.Host, name string) {
	// TODO: consider changing this to `pubsub:...`
	topic := "floodsub:" + name
	hash := u.Hash([]byte(topic))
	rz := cid.NewCidV1(cid.Raw, hash)

	err := cr.Provide(ctx, rz, true)
	if err != nil {
		log.Warningf("bootstrapPubsub: error providing rendezvous for %s: %s", topic, err.Error())
	}

	go func() {
		for {
			select {
			case <-time.After(8 * time.Hour):
				err := cr.Provide(ctx, rz, true)
				if err != nil {
					log.Warningf("bootstrapPubsub: error providing rendezvous for %s: %s", topic, err.Error())
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	rzctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	wg := &sync.WaitGroup{}
	for pi := range cr.FindProvidersAsync(rzctx, rz, 10) {
		if pi.ID == host.ID() {
			continue
		}
		wg.Add(1)
		go func(pi pstore.PeerInfo) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(ctx, time.Second*10)
			defer cancel()

			err := host.Connect(ctx, pi)
			if err != nil {
				log.Debugf("Error connecting to pubsub peer %s: %s", pi.ID, err.Error())
				return
			}

			// delay to let pubsub perform its handshake
			time.Sleep(time.Millisecond * 250)

			log.Debugf("Connected to pubsub peer %s", pi.ID)
		}(pi)
	}

	wg.Wait()
}
