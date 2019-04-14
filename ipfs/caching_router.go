package ipfs

import (
	"context"
	"gx/ipfs/QmRCrPXk2oUwpK1Cj2FXrUotRpddUxz56setkny2gz13Cx/go-libp2p-routing-helpers"
	"gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing"
	"gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing/options"
	dht "gx/ipfs/QmSY3nkMNLzh9GdbFKK5tT7YMfLpf52iUZ8ZRkr29MJaa5/go-libp2p-kad-dht"
	record "gx/ipfs/QmbeHtaBy9nZsW4cHRcvgVY4CnDhXudE2Dr6qDxS7yg9rX/go-libp2p-record"
)

type CachingRouter struct {
	apiRouter *APIRouter
	routing.IpfsRouting
	RecordValidator record.Validator
}

func NewCachingRouter(dht *dht.IpfsDHT, apiRouter *APIRouter) *CachingRouter {
	return &CachingRouter{
		apiRouter: apiRouter,
		IpfsRouting: dht,
		RecordValidator: dht.Validator,
	}
}

func (r *CachingRouter) DHT() *dht.IpfsDHT {
	return r.IpfsRouting.(*dht.IpfsDHT)
}

func (r *CachingRouter) PutValue(ctx context.Context, key string, value []byte, opts ...ropts.Option) error {
	log.Notice("Putting value...")
	// Write to the tiered router in the background then write to the caching
	// router and return
	go r.IpfsRouting.PutValue(ctx, key, value, opts...)
	return r.apiRouter.PutValue(ctx, key, value, opts...)
}

func (r *CachingRouter) GetValue(ctx context.Context, key string, opts ...ropts.Option) ([]byte, error) {
	log.Notice("Getting value...")
	// First check the tiered router. If it's successful return the value otherwise
	// continue on to check the other routers.
	val, err := r.IpfsRouting.GetValue(ctx, key, opts...)
	if err != nil {
		return nil, err
	}

	// Value miss; Check API router
	val, err = r.apiRouter.GetValue(ctx, key, opts...)
	if err == nil {
		return val, nil
	}

	// Write value back to caching router so it can hit next time.
	return val, r.apiRouter.PutValue(ctx, key, val, opts...)
}

func (r *CachingRouter) SearchValue(ctx context.Context, key string, opts ...ropts.Option) (<-chan []byte, error) {
	return routinghelpers.Parallel{
		Routers: []routing.IpfsRouting{
			r.IpfsRouting,
			r.apiRouter,
		},
		Validator: r.RecordValidator,
	}.SearchValue(ctx, key, opts...)
}
