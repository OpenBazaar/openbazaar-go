package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/OpenBazaar/multiwallet/client/blockbook"
	clientErr "github.com/OpenBazaar/multiwallet/client/errors"
	"github.com/OpenBazaar/multiwallet/model"
	"github.com/btcsuite/btcutil"
	"github.com/op/go-logging"
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

	HTTPClient http.Client
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

func (m *maxTryEnum) isFinal() bool {
	return m.attempts == m.max
}

// NewClientPool instantiates a new ClientPool object with the given server APIs
func NewClientPool(endpoints []string, proxyDialer proxy.Dialer) (*ClientPool, error) {
	if len(endpoints) == 0 {
		return nil, errors.New("no client endpoints provided")
	}

	var (
		pool = &ClientPool{
			blockChan:    make(chan model.Block),
			poolManager:  &rotationManager{},
			listenAddrs:  make([]btcutil.Address, 0),
			txChan:       make(chan model.Transaction),
			unblockStart: make(chan struct{}, 1),
		}
		manager, err = newRotationManager(endpoints, proxyDialer)
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

func (p *ClientPool) Clients() []*blockbook.BlockBookClient {
	var clients []*blockbook.BlockBookClient
	for _, c := range p.poolManager.clientCache {
		clients = append(clients, c)
	}
	return clients
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
	defer close(closeChan)
	if err := p.poolManager.StartCurrent(closeChan); err != nil {
		Log.Errorf("error starting %s: %s", p.poolManager.currentTarget, err.Error())
		p.poolManager.FailCurrent()
		p.poolManager.CloseCurrent()
		return err
	}
	var ctx context.Context
	ctx, p.cancelListenChan = context.WithCancel(context.Background())
	go p.listenChans(ctx)
	defer p.stopWebsocketListening()
	p.replayListenAddresses()
	if err := <-closeChan; err != nil {
		p.poolManager.FailCurrent()
		p.poolManager.CloseCurrent()
	}
	return nil
}

// Close proxies the same request to the active client
func (p *ClientPool) Close() {
	p.stopWebsocketListening()
	p.unblockStart <- struct{}{}
	p.poolManager.CloseCurrent()
}

// PoolManager returns the pool manager object
func (p *ClientPool) PoolManager() *rotationManager {
	return p.poolManager
}

// FailAndCloseCurrentClient cleans up the active client's connections, and
// signals to the rotation manager that it is unhealthy. The internal runLoop
// will detect the client's closing and attempt to start the next available.
func (p *ClientPool) FailAndCloseCurrentClient() {
	p.stopWebsocketListening()
	p.poolManager.FailCurrent()
	p.poolManager.CloseCurrent()
}

func (p *ClientPool) stopWebsocketListening() {
	if p.cancelListenChan != nil {
		p.cancelListenChan()
		p.cancelListenChan = nil
	}
}

// listenChans proxies the block and tx chans from the client to the ClientPool's channels
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

// executeRequest handles making the HTTP request and responding with rotating the
// pool and/or retrying requests. As all requests travel through this method, without
// middleware intercepting error responses, all returned errors are checked to see if
// they are Retryable and/or Fatal as defined by client/errors. These error properties
// can be composed like client/errors.MakeFatal(client/errors.MakeRetryable(err)).
// This approach should allow individual requests to define how the resulting error
// should be handled upstream of the request.
func (p *ClientPool) executeRequest(queryFunc func(c *blockbook.BlockBookClient) error) error {
	var err error
	for e := p.newMaximumTryEnumerator(); e.next(); {
		var client = p.poolManager.AcquireCurrentWhenReady()
		if err = queryFunc(client); err != nil {
			p.poolManager.ReleaseCurrent()
			if clientErr.IsFatal(err) || e.isFinal() {
				Log.Warningf("rotating server due to fatal or exhausted attempts")
				p.FailAndCloseCurrentClient()
			}
			if !clientErr.IsRetryable(err) {
				Log.Errorf("unretryable error: %s", err.Error())
				return err
			}
			if !e.isFinal() {
				Log.Warningf("retrying due to error: %s", err.Error())
			}
			continue
		} else {
			p.poolManager.ReleaseCurrent()
			return nil
		}
	}
	Log.Errorf("exhausted retry attempts, last error: %s", err.Error())
	return fmt.Errorf("request failed: %s", err.Error())
}

// BlockNofity proxies the active client's block channel
func (p *ClientPool) BlockNotify() <-chan model.Block {
	return p.blockChan
}

// Broadcast proxies the same request to the active client
func (p *ClientPool) Broadcast(tx []byte) (string, error) {
	var (
		txid      string
		queryFunc = func(c *blockbook.BlockBookClient) error {
			Log.Debugf("(%s) broadcasting transaction", c.EndpointURL().String())
			r, err := c.Broadcast(tx)
			if err != nil {
				return err
			}
			txid = r
			return nil
		}
	)

	err := p.executeRequest(queryFunc)
	return txid, err
}

// EstimateFee proxies the same request to the active client
func (p *ClientPool) EstimateFee(nBlocks int) (int, error) {
	var (
		fee       int
		queryFunc = func(c *blockbook.BlockBookClient) error {
			Log.Debugf("(%s) requesting fee estimate", c.EndpointURL().String())
			r, err := c.EstimateFee(nBlocks)
			if err != nil {
				return clientErr.MakeRetryable(err)
			}
			fee = r
			return nil
		}
	)

	err := p.executeRequest(queryFunc)
	return fee, err
}

// GetBestBlock proxies the same request to the active client
func (p *ClientPool) GetBestBlock() (*model.Block, error) {
	var (
		block     *model.Block
		queryFunc = func(c *blockbook.BlockBookClient) error {
			Log.Debugf("(%s) request best block info", c.EndpointURL().String())
			r, err := c.GetBestBlock()
			if err != nil {
				return clientErr.MakeRetryable(err)
			}
			block = r
			return err
		}
	)

	err := p.executeRequest(queryFunc)
	return block, err
}

// GetInfo proxies the same request to the active client
func (p *ClientPool) GetInfo() (*model.Info, error) {
	var (
		info      *model.Info
		queryFunc = func(c *blockbook.BlockBookClient) error {
			Log.Debugf("(%s) request backend info", c.EndpointURL().String())
			r, err := c.GetInfo()
			if err != nil {
				return clientErr.MakeRetryable(err)
			}
			info = r
			return nil
		}
	)

	err := p.executeRequest(queryFunc)
	return info, err
}

// GetRawTransaction proxies the same request to the active client
func (p *ClientPool) GetRawTransaction(txid string) ([]byte, error) {
	var (
		tx        []byte
		queryFunc = func(c *blockbook.BlockBookClient) error {
			Log.Debugf("(%s) request transaction info, txid: %s", c.EndpointURL().String(), txid)
			r, err := c.GetRawTransaction(txid)
			if err != nil {
				return err
			}
			tx = r
			return nil
		}
	)
	err := p.executeRequest(queryFunc)
	return tx, err
}

// GetTransactions proxies the same request to the active client
func (p *ClientPool) GetTransactions(addrs []btcutil.Address) ([]model.Transaction, error) {
	var (
		txs       []model.Transaction
		queryFunc = func(c *blockbook.BlockBookClient) error {
			var addrStrings []string
			for _, a := range addrs {
				addrStrings = append(addrStrings, a.String())
			}
			Log.Debugf("(%s) request transactions for (%d) addrs", c.EndpointURL().String(), len(addrs))
			Log.Debugf("\taddrs requested: %s", strings.Join(addrStrings, ","))
			r, err := c.GetTransactions(addrs)
			if err != nil {
				return err
			}
			txs = r
			return nil
		}
	)

	err := p.executeRequest(queryFunc)
	return txs, err
}

// GetTransaction proxies the same request to the active client
func (p *ClientPool) GetTransaction(txid string) (*model.Transaction, error) {
	var (
		tx        *model.Transaction
		queryFunc = func(c *blockbook.BlockBookClient) error {
			Log.Debugf("(%s) request transaction data, txid: %s", c.EndpointURL().String(), txid)
			r, err := c.GetTransaction(txid)
			if err != nil {
				return err
			}
			tx = r
			return nil
		}
	)

	err := p.executeRequest(queryFunc)
	return tx, err
}

// GetUtxos proxies the same request to the active client
func (p *ClientPool) GetUtxos(addrs []btcutil.Address) ([]model.Utxo, error) {
	var (
		utxos     []model.Utxo
		queryFunc = func(c *blockbook.BlockBookClient) error {
			var addrStrings []string
			for _, a := range addrs {
				addrStrings = append(addrStrings, a.String())
			}
			Log.Debugf("(%s) request utxos for (%d) addrs", c.EndpointURL().String(), len(addrs))
			Log.Debugf("\taddrs requested: %s", strings.Join(addrStrings, ","))
			r, err := c.GetUtxos(addrs)
			if err != nil {
				return err
			}
			utxos = r
			return nil
		}
	)

	err := p.executeRequest(queryFunc)
	return utxos, err
}

// ListenAddresses proxies the same request to the active client
func (p *ClientPool) ListenAddresses(addrs ...btcutil.Address) {
	p.listenAddrsLock.Lock()
	defer p.listenAddrsLock.Unlock()
	var client = p.poolManager.AcquireCurrentWhenReady()
	defer p.poolManager.ReleaseCurrent()

	p.listenAddrs = append(p.listenAddrs, addrs...)
	client.ListenAddresses(addrs...)
}

func (p *ClientPool) replayListenAddresses() {
	p.listenAddrsLock.Lock()
	defer p.listenAddrsLock.Unlock()
	var client = p.poolManager.AcquireCurrent()
	defer p.poolManager.ReleaseCurrent()
	client.ListenAddresses(p.listenAddrs...)
}

// TransactionNotify proxies the active client's tx channel
func (p *ClientPool) TransactionNotify() <-chan model.Transaction { return p.txChan }
