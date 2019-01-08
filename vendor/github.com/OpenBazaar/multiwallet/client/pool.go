package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"sync"

	"github.com/OpenBazaar/multiwallet/client/blockbook"
	"github.com/OpenBazaar/multiwallet/model"
	"github.com/btcsuite/btcutil"
	logging "github.com/op/go-logging"
	"golang.org/x/net/proxy"
)

var Log = logging.MustGetLogger("pool")

// ClientPool is an implementation of the APIClient interface which will handle
// server failure, rotate servers, and retry API requests.
type ClientPool struct {
	blockChan        chan model.Block
	cancelListenChan context.CancelFunc
	listenAddrs      []btcutil.Address
	listenAddrsLock  sync.Mutex
	poolManager      *rotationManager
	proxyDialer      proxy.Dialer
	txChan           chan model.Transaction
	unblockStart     chan struct{}

	HTTPClient  http.Client
	ClientCache []*blockbook.BlockBookClient
}

func (p *ClientPool) newMaximumTryEnumerator() *maxTryEnum {
	return &maxTryEnum{max: 3, attempts: 0}
}

type maxTryEnum struct{ max, attempts int }

func (m *maxTryEnum) next() bool {
	var now = m.attempts
	m.attempts++
	return now < m.max
}

// NewClientPool instantiates a new ClientPool object with the given server APIs
func NewClientPool(endpoints []string, proxyDialer proxy.Dialer) (*ClientPool, error) {
	if len(endpoints) == 0 {
		return nil, errors.New("no client endpoints provided")
	}

	var (
		clientCache = make([]*blockbook.BlockBookClient, len(endpoints))
		pool        = &ClientPool{
			blockChan:    make(chan model.Block),
			poolManager:  &rotationManager{},
			listenAddrs:  make([]btcutil.Address, 0),
			txChan:       make(chan model.Transaction),
			unblockStart: make(chan struct{}, 1),
			ClientCache:  clientCache,
		}
		manager, err = newRotationManager(endpoints, proxyDialer, pool.doRequest)
	)
	if err != nil {
		return nil, err
	}
	pool.poolManager = manager
	return pool, nil
}

// Start will attempt to connect to the first available server. If it fails to
// connect it will rotate through the servers to try to find one that works.
func (p *ClientPool) Start() error {
	go p.run()
	return nil
}

func (p *ClientPool) run() {
	for {
		select {
		case <-p.unblockStart:
			return
		default:
			p.runLoop()
		}
	}
}

func (p *ClientPool) runLoop() error {
	p.poolManager.SelectNext()
	var closeChan = make(chan error, 0)
	if err := p.poolManager.StartCurrent(closeChan); err != nil {
		Log.Errorf("error starting %s: %s", p.poolManager.currentTarget, err.Error())
		p.poolManager.FailCurrent()
		p.poolManager.CloseCurrent()
		return err
	}
	var ctx context.Context
	ctx, p.cancelListenChan = context.WithCancel(context.Background())
	go p.listenChans(ctx)
	p.replayListenAddresses()
	if err := <-closeChan; err != nil {
		p.poolManager.FailCurrent()
		p.poolManager.CloseCurrent()
	}
	return nil
}

// Close proxies the same request to the active InsightClient
func (p *ClientPool) Close() {
	if p.cancelListenChan != nil {
		p.cancelListenChan()
		p.cancelListenChan = nil
	}
	p.unblockStart <- struct{}{}
	p.poolManager.CloseCurrent()
}

// FailAndCloseCurrentClient cleans up the active client's connections, and
// signals to the rotation manager that it is unhealthy. The internal runLoop
// will detect the client's closing and attempt to start the next available.
func (p *ClientPool) FailAndCloseCurrentClient() {
	if p.cancelListenChan != nil {
		p.cancelListenChan()
		p.cancelListenChan = nil
	}
	p.poolManager.FailCurrent()
	p.poolManager.CloseCurrent()
}

// listenChans proxies the block and tx chans from the InsightClient to the ClientPool's channels
func (p *ClientPool) listenChans(ctx context.Context) {
	var (
		client    = p.poolManager.AcquireCurrent()
		blockChan = client.BlockChannel()
		txChan    = client.TxChannel()
	)
	defer p.poolManager.ReleaseCurrent()
	go func() {
		for {
			select {
			case block := <-blockChan:
				p.blockChan <- block
			case tx := <-txChan:
				p.txChan <- tx
			case <-ctx.Done():
				return
			}
		}
	}()
}

// doRequest handles making the HTTP request with server rotation and retires. Only if all servers return an
// error will this method return an error.
func (p *ClientPool) doRequest(endpoint, method string, body []byte, query url.Values) (*http.Response, error) {
	for e := p.newMaximumTryEnumerator(); e.next(); {
		var client = p.poolManager.AcquireCurrentWhenReady()
		requestUrl := client.EndpointURL()
		requestUrl.Path = path.Join(client.EndpointURL().Path, endpoint)
		req, err := http.NewRequest(method, requestUrl.String(), bytes.NewReader(body))
		if query != nil {
			req.URL.RawQuery = query.Encode()
		}
		if err != nil {
			Log.Errorf("error preparing request (%s %s)", method, requestUrl.String())
			Log.Errorf("\terror continued: %s", err.Error())
			p.poolManager.ReleaseCurrent()
			return nil, fmt.Errorf("invalid request: %s", err)
		}
		req.Header.Add("Content-Type", "application/json")

		resp, err := p.HTTPClient.Do(req)
		if err != nil {
			Log.Errorf("error making request (%s %s)", method, requestUrl.String())
			Log.Errorf("\terror continued: %s", err.Error())
			p.poolManager.ReleaseCurrent()
			p.FailAndCloseCurrentClient()
			continue
		}
		// Try again if for some reason it returned a bad request
		if resp.StatusCode == http.StatusBadRequest {
			// Reset the body so we can read it again.
			req.Body = ioutil.NopCloser(bytes.NewReader(body))
			resp, err = p.HTTPClient.Do(req)
			if err != nil {
				Log.Errorf("error making request (%s %s)", method, requestUrl.String())
				Log.Errorf("\terror continued: %s", err.Error())
				p.poolManager.ReleaseCurrent()
				p.FailAndCloseCurrentClient()
				continue
			}
		}
		if resp.StatusCode != http.StatusOK {
			p.poolManager.ReleaseCurrent()
			p.FailAndCloseCurrentClient()
			continue
		}
		p.poolManager.ReleaseCurrent()
		return resp, nil
	}
	return nil, errors.New("exhausted maximum attempts for request")
}

// BlockNofity proxies the active InsightClient's block channel
func (p *ClientPool) BlockNotify() <-chan model.Block {
	return p.blockChan
}

// Broadcast proxies the same request to the active InsightClient
func (p *ClientPool) Broadcast(tx []byte) (string, error) {
	var client = p.poolManager.AcquireCurrentWhenReady()
	defer p.poolManager.ReleaseCurrent()
	return client.Broadcast(tx)
}

// EstimateFee proxies the same request to the active InsightClient
func (p *ClientPool) EstimateFee(nBlocks int) (int, error) {
	var client = p.poolManager.AcquireCurrentWhenReady()
	defer p.poolManager.ReleaseCurrent()
	return client.EstimateFee(nBlocks)
}

// GetBestBlock proxies the same request to the active InsightClient
func (p *ClientPool) GetBestBlock() (*model.Block, error) {
	var client = p.poolManager.AcquireCurrentWhenReady()
	defer p.poolManager.ReleaseCurrent()
	return client.GetBestBlock()
}

// GetInfo proxies the same request to the active InsightClient
func (p *ClientPool) GetInfo() (*model.Info, error) {
	var client = p.poolManager.AcquireCurrentWhenReady()
	defer p.poolManager.ReleaseCurrent()
	return client.GetInfo()
}

// GetRawTransaction proxies the same request to the active InsightClient
func (p *ClientPool) GetRawTransaction(txid string) ([]byte, error) {
	var client = p.poolManager.AcquireCurrentWhenReady()
	defer p.poolManager.ReleaseCurrent()
	return client.GetRawTransaction(txid)
}

// GetTransactions proxies the same request to the active InsightClient
func (p *ClientPool) GetTransactions(addrs []btcutil.Address) ([]model.Transaction, error) {
	var client = p.poolManager.AcquireCurrentWhenReady()
	defer p.poolManager.ReleaseCurrent()
	return client.GetTransactions(addrs)
}

// GetTransaction proxies the same request to the active InsightClient
func (p *ClientPool) GetTransaction(txid string) (*model.Transaction, error) {
	var client = p.poolManager.AcquireCurrentWhenReady()
	defer p.poolManager.ReleaseCurrent()
	return client.GetTransaction(txid)
}

// GetUtxos proxies the same request to the active InsightClient
func (p *ClientPool) GetUtxos(addrs []btcutil.Address) ([]model.Utxo, error) {
	var client = p.poolManager.AcquireCurrentWhenReady()
	defer p.poolManager.ReleaseCurrent()
	return client.GetUtxos(addrs)
}

// ListenAddress proxies the same request to the active InsightClient
func (p *ClientPool) ListenAddress(addr btcutil.Address) {
	p.listenAddrsLock.Lock()
	defer p.listenAddrsLock.Unlock()
	var client = p.poolManager.AcquireCurrentWhenReady()
	defer p.poolManager.ReleaseCurrent()
	p.listenAddrs = append(p.listenAddrs, addr)
	client.ListenAddress(addr)
}

func (p *ClientPool) replayListenAddresses() {
	p.listenAddrsLock.Lock()
	defer p.listenAddrsLock.Unlock()
	var client = p.poolManager.AcquireCurrent()
	defer p.poolManager.ReleaseCurrent()
	for _, addr := range p.listenAddrs {
		client.ListenAddress(addr)
	}
}

// TransactionNotify proxies the active InsightClient's tx channel
func (p *ClientPool) TransactionNotify() <-chan model.Transaction { return p.txChan }
