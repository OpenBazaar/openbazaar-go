package blockstackclient

import (
	"encoding/json"
	"errors"
	"golang.org/x/net/proxy"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
	"context"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
)

type httpClient interface {
	Get(string) (*http.Response, error)
}

type BlockstackClient struct {
	resolverURL string
	httpClient  httpClient
	sync.Mutex
}

func NewBlockStackClient(resolverURL string, dialer proxy.Dialer) *BlockstackClient {
	dial := net.Dial
	if dialer != nil {
		dial = dialer.Dial
	}
	tbTransport := &http.Transport{Dial: dial}
	client := &http.Client{Transport: tbTransport, Timeout: time.Minute}
	b := &BlockstackClient{
		resolverURL: resolverURL,
		httpClient:  client,
	}
	return b
}

type lookupRes struct {
	pid  peer.ID
	error error
}

func (b *BlockstackClient) Resolve(ctx context.Context, name string) (pid peer.ID, err error) {
	if !strings.HasSuffix(name, ".id") {
		return pid, errors.New("Domain is not a blockstack domain")
	}

	split := strings.Split(name, ".")
	name = split[0]

	rootChan := make(chan lookupRes, 1)
	go workDomain(b, name, rootChan)

	var rootRes lookupRes
	select {
	case rootRes = <-rootChan:
	case <-ctx.Done():
		return pid, ctx.Err()
	}
	if rootRes.error == nil {
		pid = rootRes.pid
	} else {
		return pid, errors.New("Not found")
	}
	return pid, nil
}

func (b *BlockstackClient) Domains() []string {
	return []string{"id"}
}

func workDomain(b *BlockstackClient, name string, res chan lookupRes) {
	var pid peer.ID
	defer func() {
		if rerr := recover(); rerr != nil {
			res <- lookupRes{pid, errors.New("Couldn't parse blockchainID json")}
			return
		}
	}()
	resolver, err := url.Parse(b.resolverURL)
	if err != nil {
		res <- lookupRes{pid, err}
		return
	}
	resolver.Path = path.Join(resolver.Path, "v2", "users", name)

	resp, err := b.httpClient.Get(resolver.String())
	if err != nil {
		res <- lookupRes{pid, errors.New("Error querying resolver")}
		return
	}
	if resp.StatusCode == http.StatusNotFound {
		res <- lookupRes{pid, errors.New("Handle not found")}
	}
	decoder := json.NewDecoder(resp.Body)
	var data map[string]interface{}
	err = decoder.Decode(&data)
	if err != nil {
		res <- lookupRes{pid, err}
		return
	}
	obj := data[name].(map[string]interface{})
	profile := obj["profile"].(map[string]interface{})
	account := profile["account"].([]interface{})
	for _, a := range account {
		acc := a.(map[string]interface{})
		service := strings.ToLower(acc["service"].(string))
		identifier := acc["identifier"].(string)
		if service == "openbazaar" {
			pid, err := peer.IDB58Decode(identifier)
			if err != nil {
				res <- lookupRes{pid, err}
				return
			}
			res <- lookupRes{pid, nil}
			return
		}
	}
}
