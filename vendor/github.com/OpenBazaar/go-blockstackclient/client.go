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
	"gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
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

func (b *BlockstackClient) Resolve(ctx context.Context, name string) (pid peer.ID, err error) {
	defer func() {
		if rerr := recover(); rerr != nil {
			err = errors.New("Couldn't parse blockchainID json")
		}
	}()

	if !strings.HasSuffix(name, ".id") {
		return pid, errors.New("Domain is not a blockstack domain")
	}

	split := strings.Split(name, ".")
	name = split[0]

	resolver, err := url.Parse(b.resolverURL)
	if err != nil {
		return pid, err
	}
	resolver.Path = path.Join(resolver.Path, "v2", "users", name)
	resp, err := b.httpClient.Get(resolver.String())
	if err != nil {
		return pid, errors.New("Error querying resolver")
	}
	if resp.StatusCode == http.StatusNotFound {
		return pid, errors.New("Handle not found")
	}
	decoder := json.NewDecoder(resp.Body)
	var data map[string]interface{}
	err = decoder.Decode(&data)
	if err != nil {
		return pid, err
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
				return pid, err
			}
			return pid, nil
		}
	}
	return pid, errors.New("Handle does not exist")
}

func (b *BlockstackClient) Domains() []string {
	return []string{"id"}
}
