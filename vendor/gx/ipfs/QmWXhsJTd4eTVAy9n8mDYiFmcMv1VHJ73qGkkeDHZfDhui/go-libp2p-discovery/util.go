package discovery

import (
	"context"
	"time"

	pstore "gx/ipfs/QmaCTz9RkrU13bm9kMB54f7atgqM4qkjDZpRwRoJiWXEqs/go-libp2p-peerstore"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
)

var log = logging.Logger("discovery")

// FindPeers is a utility function that synchonously collects peers from a Discoverer
func FindPeers(ctx context.Context, d Discoverer, ns string, limit int) ([]pstore.PeerInfo, error) {
	res := make([]pstore.PeerInfo, 0, limit)

	log.Debugf("discovering on %s", ns)
	ch, err := d.FindPeers(ctx, ns, Limit(limit))
	if err != nil {
		return nil, err
	}

	for pi := range ch {
		log.Debugf("discovered peer on %s", ns)
		res = append(res, pi)
	}

	return res, nil
}

// Advertise is a utility function that persistently advertises a service through an Advertiser
func Advertise(ctx context.Context, a Advertiser, ns string) {
	go func() {
		for {
			ttl, err := a.Advertise(ctx, ns)
			if err != nil {
				log.Debugf("Error advertising %s: %s", ns, err.Error())
				if ctx.Err() != nil {
					return
				}

				select {
				case <-time.After(2 * time.Minute):
					continue
				case <-ctx.Done():
					return
				}
			}

			wait := 7 * ttl / 8
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return
			}
		}
	}()
}
