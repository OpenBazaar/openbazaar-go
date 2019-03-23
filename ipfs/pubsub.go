package ipfs

import (
	"context"
	"errors"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
	"gx/ipfs/QmY4dowpPFCBsbaoaJc9mNWso64eDJsm32LJznwPNaAiJG/go-libp2p-pubsub"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	routing "gx/ipfs/QmcQ81jSyWCp1jpkQ8CMbtpXT3jK7Wg6ZtYmoyWFgBoF9c/go-libp2p-routing"
	p2phost "gx/ipfs/QmdJfsSbKSZnMkfZ1kpopiyB9i3Hd6cp8VKWZmtWPa7Moc/go-libp2p-host"

	"sync"
	"time"
)

const (
	MessageTopicPrefix = "/offlinemessage/"
	GlobalIPNSTopic    = "IPNS"
	GlobalBlockTopic   = "BLOCK"
	GlobalCIDTopic     = "CID"
)

type Pubsub struct {
	Subscriber *PubsubSubscriber
	Publisher  *PubsubPublisher
}

// PubsubPublisher is a publisher that distributes arbitrary data through pubsub
type PubsubPublisher struct {
	ctx  context.Context
	ds   ds.Datastore
	host p2phost.Host
	cr   routing.ContentRouting
	ps   *pubsub.PubSub

	mx   sync.Mutex
	subs map[string]struct{}
}

// PubsubSubscriber subscribes to arbitrary subscriptions through pubsub
type PubsubSubscriber struct {
	ctx  context.Context
	ds   ds.Datastore
	host p2phost.Host
	cr   routing.ContentRouting
	ps   *pubsub.PubSub

	mx   sync.Mutex
	subs map[string]*pubsub.Subscription
}

// NewPubsubPublisher constructs a new Publisher that publishes arbitrary data through pubsub.
func NewPubsubPublisher(ctx context.Context, host p2phost.Host, cr routing.ContentRouting, ds ds.Datastore, ps *pubsub.PubSub) *PubsubPublisher {
	return &PubsubPublisher{
		ctx:  ctx,
		ds:   ds,
		host: host, // needed for pubsub bootstrap
		cr:   cr,   // needed for pubsub bootstrap
		ps:   ps,
		subs: make(map[string]struct{}),
	}
}

// NewPubsubSubscriber constructs a new subscriber for arbitrary subscriptions through pubsub.
// same as above for pubsub bootstrap dependencies
func NewPubsubSubscriber(ctx context.Context, host p2phost.Host, cr routing.ContentRouting, ds ds.Datastore, ps *pubsub.PubSub) *PubsubSubscriber {
	return &PubsubSubscriber{
		ctx:  ctx,
		ds:   ds,
		host: host, // needed for pubsub bootstrap
		cr:   cr,   // needed for pubsub bootstrap
		ps:   ps,
		subs: make(map[string]*pubsub.Subscription),
	}
}

func (p *PubsubPublisher) Publish(ctx context.Context, topic string, data []byte) error {
	p.mx.Lock()
	_, ok := p.subs[topic]

	if !ok {
		p.subs[topic] = struct{}{}
		p.mx.Unlock()

		bootstrapPubsub(p.ctx, p.cr, p.host, topic)
	} else {
		p.mx.Unlock()
	}

	log.Debugf("PubsubPublish: publish data for %s", topic)
	return p.ps.Publish(topic, data)
}

func (r *PubsubSubscriber) Subscribe(ctx context.Context, topic string) (chan []byte, error) {
	r.mx.Lock()
	// see if we already have a pubsub subscription; if not, subscribe
	_, ok := r.subs[topic]
	resp := make(chan []byte)
	if !ok {
		sub, err := r.ps.Subscribe(topic)
		if err != nil {
			r.mx.Unlock()
			return nil, err
		}

		log.Debugf("PubsubSubscribe: subscribed to %s", topic)

		r.subs[topic] = sub

		ctx, cancel := context.WithCancel(r.ctx)
		go r.handleSubscription(sub, topic, resp, cancel)
		go bootstrapPubsub(ctx, r.cr, r.host, topic)
	}
	r.mx.Unlock()
	return resp, nil
}

// GetSubscriptions retrieves a list of active topic subscriptions
func (r *PubsubSubscriber) GetSubscriptions() []string {
	r.mx.Lock()
	defer r.mx.Unlock()

	var res []string
	for sub := range r.subs {
		res = append(res, sub)
	}

	return res
}

// Cancel cancels a topic subscription; returns true if an active
// subscription was canceled
func (r *PubsubSubscriber) Cancel(name string) bool {
	r.mx.Lock()
	defer r.mx.Unlock()

	sub, ok := r.subs[name]
	if ok {
		sub.Cancel()
		delete(r.subs, name)
	}

	return ok
}

func (r *PubsubSubscriber) handleSubscription(sub *pubsub.Subscription, topic string, resp chan<- []byte, cancel func()) {
	defer sub.Cancel()
	defer cancel()

	for {
		msg, err := sub.Next(r.ctx)
		if err != nil {
			if err != context.Canceled {
				log.Warningf("PubsubSubscribe: subscription error in %s: %s", topic, err.Error())
			}
			return
		}

		err = r.receive(msg, topic, resp)
		if err != nil {
			log.Warningf("PubsubSubscribe: error processing update for %s: %s", topic, err.Error())
		}
	}
}

func (r *PubsubSubscriber) receive(msg *pubsub.Message, topic string, resp chan<- []byte) error {
	data := msg.GetData()
	if data == nil {
		return errors.New("empty message")
	}

	log.Debugf("PubsubSubscribe: receive data for topic %s", topic)

	resp <- data
	return nil
}

// rendezvous with peers in the name topic through provider records
// Note: rendezvous/boostrap should really be handled by the pubsub implementation itself!
func bootstrapPubsub(ctx context.Context, cr routing.ContentRouting, host p2phost.Host, topic string) {
	topic = "floodsub:" + topic
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
