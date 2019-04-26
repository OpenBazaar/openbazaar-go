package ipfs

import (
	"context"
	"errors"
	"runtime/debug"

	dht "gx/ipfs/QmSY3nkMNLzh9GdbFKK5tT7YMfLpf52iUZ8ZRkr29MJaa5/go-libp2p-kad-dht"
	routing "gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing"
	ropts "gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing/options"
	record "gx/ipfs/QmbeHtaBy9nZsW4cHRcvgVY4CnDhXudE2Dr6qDxS7yg9rX/go-libp2p-record"
)

var (
	ErrCachingRouterIncorrectRoutingType = errors.New("Incorrect routing type")
)

type CachingRouter struct {
	apiRouter *APIRouter
	routing.IpfsRouting
	RecordValidator record.Validator
}

func NewCachingRouter(dht *dht.IpfsDHT, apiRouter *APIRouter) *CachingRouter {
	return &CachingRouter{
		apiRouter:       apiRouter,
		IpfsRouting:     dht,
		RecordValidator: dht.Validator,
	}
}

func (r *CachingRouter) DHT() (*dht.IpfsDHT, error) {
	dht, ok := r.IpfsRouting.(*dht.IpfsDHT)
	if !ok {
		return nil, ErrCachingRouterIncorrectRoutingType
	}
	return dht, nil
}

func (r *CachingRouter) APIRouter() *APIRouter {
	return r.apiRouter
}

func (r *CachingRouter) PutValue(ctx context.Context, key string, value []byte, opts ...ropts.Option) error {
	log.Debugf("PutValue")
	// Write to the tiered router in the background then write to the caching
	// router and return
	if err := r.IpfsRouting.PutValue(ctx, key, value, opts...); err != nil {
		log.Errorf("put ipfs routing (%s): %s", key, err)
	}
	if err := r.apiRouter.PutValue(ctx, key, value, opts...); err != nil {
		log.Errorf("put API routing (%s): %s", key, err)
	}
	return nil
}

func (r *CachingRouter) GetValue(ctx context.Context, key string, opts ...ropts.Option) ([]byte, error) {
	// First check the DHT router. If it's successful return the value otherwise
	// continue on to check the other routers.
	log.Debugf("GetValue")
	val, err := r.IpfsRouting.GetValue(ctx, key, opts...)
	if err == nil {
		if err := r.apiRouter.PutValue(ctx, key, val, opts...); err != nil {
			log.Errorf("DHT put value error: %s", err.Error())
			return nil, err
		}
		return val, nil
	} else {
		if len(val) > 0 {
			log.Errorf("found value despite getting error: %s", err)
			return val, nil
		}
		log.Errorf("DHT get value error: %s", err.Error())
		log.Infof("failing back to API router")
	}

	// Value miss; Check API router
	return r.apiRouter.GetValue(ctx, key, opts...)
}

func (r *CachingRouter) SearchValue(ctx context.Context, key string, opts ...ropts.Option) (<-chan []byte, error) {
	log.Debugf("SearchValue: %s", key)
	debug.PrintStack()
	return r.IpfsRouting.SearchValue(ctx, key, opts...)
	//return routinghelpers.Parallel{
	//	Routers: []routing.IpfsRouting{
	//		r.IpfsRouting,
	//		r.apiRouter,
	//	},
	//	Validator: r.RecordValidator,
	//}.SearchValue(ctx, key, opts...)
}
