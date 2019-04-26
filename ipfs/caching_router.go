package ipfs

import (
	"context"
	"encoding/hex"
	"errors"

	//routinghelpers "gx/ipfs/QmRCrPXk2oUwpK1Cj2FXrUotRpddUxz56setkny2gz13Cx/go-libp2p-routing-helpers"
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
	// Write to the tiered router in the background then write to the caching
	// router and return
	var err error
	if err = r.IpfsRouting.PutValue(ctx, key, value, opts...); err != nil {
		log.Errorf("ipfs dht put (%s): %s", hex.EncodeToString([]byte(key)), err)
		return err
	}
	if err = r.apiRouter.PutValue(ctx, key, value, opts...); err != nil {
		log.Errorf("api cache put (%s): %s", hex.EncodeToString([]byte(key)), err)
	}
	return err
}

func (r *CachingRouter) GetValue(ctx context.Context, key string, opts ...ropts.Option) ([]byte, error) {
	// First check the DHT router. If it's successful return the value otherwise
	// continue on to check the other routers.
	val, err := r.IpfsRouting.GetValue(ctx, key, opts...)
	if err != nil && len(val) == 0 {
		// No values from the DHT, check the API cache
		log.Warningf("ipfs dht lookup was empty: %s", err.Error())
		if val, err = r.apiRouter.GetValue(ctx, key, opts...); err != nil && len(val) == 0 {
			// No values still, report NotFound
			return nil, routing.ErrNotFound
		}
	}
	if err := r.apiRouter.PutValue(ctx, key, val, opts...); err != nil {
		log.Errorf("api cache put found dht value (%s): %s", hex.EncodeToString([]byte(key)), err.Error())
	}
	return val, nil
}

func (r *CachingRouter) SearchValue(ctx context.Context, key string, opts ...ropts.Option) (<-chan []byte, error) {
	// TODO: Restore parallel lookup once validation is properly applied to
	// the apiRouter results ensuring it doesn't return invalid records before the
	// IpfsRouting object can. For some reason the validation is not being considered
	// on returned results.
	return r.IpfsRouting.SearchValue(ctx, key, opts...)
	//return routinghelpers.Parallel{
	//Routers: []routing.IpfsRouting{
	//r.IpfsRouting,
	//r.apiRouter,
	//},
	//Validator: r.RecordValidator,
	//}.SearchValue(ctx, key, opts...)
}
