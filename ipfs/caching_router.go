package ipfs

import (
	"context"
	"errors"
	"gx/ipfs/QmRCrPXk2oUwpK1Cj2FXrUotRpddUxz56setkny2gz13Cx/go-libp2p-routing-helpers"
	"gx/ipfs/QmSY3nkMNLzh9GdbFKK5tT7YMfLpf52iUZ8ZRkr29MJaa5/go-libp2p-kad-dht"
	"gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing"
	"gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing/options"
)

var (
	ErrCachingRouterIncorrectRoutingType = errors.New("Incorrect routing type")
)

type CachingRouter struct {
	apiRouter    *APIRouter
	tieredRouter routinghelpers.Tiered
	routing.IpfsRouting
}

func NewCachingRouter(dht *dht.IpfsDHT, apiRouter *APIRouter) *CachingRouter {
	tierd := routinghelpers.Tiered{
		Routers: []routing.IpfsRouting{
			dht,
			apiRouter,
		},
		Validator: dht.Validator,
	}

	return &CachingRouter{
		apiRouter:       apiRouter,
		tieredRouter:    tierd,
		IpfsRouting:     dht,
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
	return r.tieredRouter.PutValue(ctx, key, value, opts...)
}

func (r *CachingRouter) GetValue(ctx context.Context, key string, opts ...ropts.Option) ([]byte, error) {
	value, err := r.tieredRouter.GetValue(ctx, key, opts...)
	if err == nil {
		go r.apiRouter.PutValue(ctx, key, value, opts...)
	}
	return value, err
}

func (r *CachingRouter) SearchValue(ctx context.Context, key string, opts ...ropts.Option) (<-chan []byte, error) {
	return r.tieredRouter.SearchValue(ctx, key, opts...)
}
