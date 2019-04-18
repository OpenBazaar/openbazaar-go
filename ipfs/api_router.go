package ipfs

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"golang.org/x/net/proxy"
	"io/ioutil"
	"net/http"
	"time"

	"gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	routing "gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing"
	ropts "gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing/options"
	pstore "gx/ipfs/QmaCTz9RkrU13bm9kMB54f7atgqM4qkjDZpRwRoJiWXEqs/go-libp2p-peerstore"
)

var apiRouterHTTPClient = &http.Client{
	Timeout: time.Second * 30,
}

// ensure APIRouter satisfies the interface
var _ routing.IpfsRouting = &APIRouter{}

// ErrNotStarted is returned if a method is called before the router
// is started using the Start() method.
var ErrNotStarted = errors.New("API router not started")

// APIRouter is a routing.IpfsRouting compliant struct backed by an API. It only
// provides the features offerened by routing.ValueStore and marks the others as
// unsupported.
type APIRouter struct {
	uri     string
	started chan (struct{})
}

// NewAPIRouter creates a new APIRouter backed by the given URI.
func NewAPIRouter(uri string) APIRouter {
	return APIRouter{uri: uri, started: make(chan (struct{}))}
}

func (r *APIRouter) Start(proxyDialer proxy.Dialer) {
	if proxyDialer != nil {
		tbTransport := &http.Transport{Dial: proxyDialer.Dial}
		apiRouterHTTPClient.Transport = tbTransport
	}
	close(r.started)
}

// Bootstrap is a no-op. We don't need any setup to query the API.
func (r APIRouter) Bootstrap(_ context.Context) error {
	return nil
}

// PutValue writes the given value to the API for the given key
func (r APIRouter) PutValue(ctx context.Context, key string, value []byte, opts ...ropts.Option) error {
	<-r.started
	path := r.pathForKey(key)
	req, err := http.NewRequest("PUT", path, bytes.NewBuffer(value))
	if err != nil {
		return err
	}

	log.Debugf("write value to %s", path)
	_, err = apiRouterHTTPClient.Do(req)
	return err
}

// GetValue reads the value for the given key
func (r APIRouter) GetValue(ctx context.Context, key string, opts ...ropts.Option) ([]byte, error) {
	<-r.started
	path := r.pathForKey(key)
	resp, err := apiRouterHTTPClient.Get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	log.Debugf("read value from %s", path)
	return ioutil.ReadAll(resp.Body)
}

// GetValues reads the value for the given key. The API does not return multiple
// values.
func (r APIRouter) GetValues(ctx context.Context, key string, opts ...ropts.Option) ([]byte, error) {
	<-r.started
	return r.GetValue(ctx, key, opts...)
}

// SearchValue returns the value for the given key. It return either an error or
// a closed channel containing one value.
func (r APIRouter) SearchValue(ctx context.Context, key string, opts ...ropts.Option) (<-chan []byte, error) {
	value, err := r.GetValue(ctx, key, opts...)
	if err != nil {
		return nil, err
	}

	valueCh := make(chan []byte, 1)
	valueCh <- value
	close(valueCh)
	return valueCh, nil
}

// FindPeer is unsupported
func (r APIRouter) FindPeer(_ context.Context, id peer.ID) (pstore.PeerInfo, error) {
	return pstore.PeerInfo{}, routing.ErrNotSupported
}

// FindProvidersAsync is unsupported
func (r APIRouter) FindProvidersAsync(_ context.Context, _ cid.Cid, _ int) <-chan pstore.PeerInfo {
	return nil
}

// Provide is unsupported
func (r APIRouter) Provide(_ context.Context, _ cid.Cid, _ bool) error {
	return routing.ErrNotSupported
}

func (r APIRouter) pathForKey(key string) string {
	return r.uri + "/value/" + base64.URLEncoding.EncodeToString([]byte(key))
}
