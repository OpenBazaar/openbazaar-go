package blockbook

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	gosocketio "github.com/OpenBazaar/golang-socketio"
	"github.com/OpenBazaar/golang-socketio/protocol"
	clientErr "github.com/OpenBazaar/multiwallet/client/errors"
	"github.com/OpenBazaar/multiwallet/client/transport"
	"github.com/OpenBazaar/multiwallet/model"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
	"github.com/cpacia/bchutil"
	"github.com/op/go-logging"
	"golang.org/x/net/proxy"
)

const maxInfightQueries = 25

var Log = logging.MustGetLogger("client")

type wsWatchdog struct {
	client    *BlockBookClient
	done      chan struct{}
	wsStopped chan struct{}
}

func newWebsocketWatchdog(client *BlockBookClient) *wsWatchdog {
	return &wsWatchdog{
		client:    client,
		done:      make(chan struct{}, 0),
		wsStopped: make(chan struct{}, 1),
	}
}

func (w *wsWatchdog) guardWebsocket() {
	var t = time.NewTicker(1 * time.Second)
	defer t.Stop()
	for range t.C {
		select {
		case <-w.wsStopped:
			Log.Warningf("reconnecting stopped websocket (%s)", w.client.String())
			w.client.socketMutex.Lock()
			if w.client.SocketClient != nil {
				w.client.SocketClient.Close()
				w.client.SocketClient = nil
			}
			w.drainAndRollover()
			if err := w.client.setupListeners(); err != nil {
				Log.Warningf("failed reconnecting websocket (%s)", w.client.String())
				w.client.socketMutex.Unlock()
				w.client.sendAndDiscardCloseChan(fmt.Errorf("websocket unavailable (%s): %s", w.client.String(), err.Error()))
				go w.putAway()
				return
			}
			w.client.socketMutex.Unlock()
		case <-w.done:
			return
		}
	}
}

func (w *wsWatchdog) drainAndRollover() {
	for {
		select {
		case <-w.wsStopped:
		default:
			return
		}
	}
}

func (w *wsWatchdog) bark() {
	select {
	case w.wsStopped <- struct{}{}:
	default:
	}
}

func (w *wsWatchdog) putAway() {
	select {
	case w.done <- struct{}{}:
	default:
	}
}

type BlockBookClient struct {
	apiUrl            *url.URL
	blockNotifyChan   chan model.Block
	closeChan         chan<- error
	listenLock        sync.Mutex
	listenQueue       []string
	proxyDialer       proxy.Dialer
	txNotifyChan      chan model.Transaction
	websocketWatchdog *wsWatchdog

	HTTPClient   http.Client
	RequestFunc  func(endpoint, method string, body []byte, query url.Values) (*http.Response, error)
	SocketClient model.SocketClient
	socketMutex  sync.RWMutex
}

func NewBlockBookClient(apiUrl string, proxyDialer proxy.Dialer) (*BlockBookClient, error) {
	u, err := url.Parse(apiUrl)
	if err != nil {
		return nil, err
	}

	if err := validateScheme(u); err != nil {
		return nil, err
	}

	var customClient http.Client
	if proxyDialer != nil {
		dial := proxyDialer.Dial
		tbTransport := &http.Transport{Dial: dial}
		customClient = http.Client{Timeout: time.Second * 30, Transport: tbTransport}
	} else {
		customClient = http.Client{Timeout: time.Second * 30}
	}

	bch := make(chan model.Block)
	tch := make(chan model.Transaction)

	ic := &BlockBookClient{
		HTTPClient:      customClient,
		apiUrl:          u,
		proxyDialer:     proxyDialer,
		blockNotifyChan: bch,
		txNotifyChan:    tch,
		listenLock:      sync.Mutex{},
	}
	ic.websocketWatchdog = newWebsocketWatchdog(ic)
	ic.RequestFunc = ic.doRequest
	return ic, nil
}

func (i *BlockBookClient) String() string {
	return i.apiUrl.Host
}

func (i *BlockBookClient) BlockChannel() chan model.Block {
	return i.blockNotifyChan
}

func (i *BlockBookClient) TxChannel() chan model.Transaction {
	return i.txNotifyChan
}

func (i *BlockBookClient) EndpointURL() *url.URL {
	var u = *i.apiUrl
	return &u
}

func (i *BlockBookClient) Start(closeChan chan<- error) error {
	i.socketMutex.Lock()
	defer i.socketMutex.Unlock()
	if err := i.setupListeners(); err != nil {
		return err
	}
	i.closeChan = closeChan
	return nil
}

func (i *BlockBookClient) Close() {
	Log.Infof("closing client (%s)...", i.String())
	i.socketMutex.Lock()
	defer i.socketMutex.Unlock()
	if i.SocketClient != nil {
		if i.websocketWatchdog != nil {
			go i.websocketWatchdog.putAway()
		}
		i.SocketClient.Close()
		i.SocketClient = nil
	}
	i.sendAndDiscardCloseChan(nil)
}

func (i *BlockBookClient) sendAndDiscardCloseChan(err error) {
	if i.closeChan != nil {
		i.closeChan <- err
		i.closeChan = nil
	}
}

func validateScheme(target *url.URL) error {
	switch target.Scheme {
	case "https", "http":
		return nil
	}
	return fmt.Errorf("unsupported scheme: %s", target.Scheme)
}

func (i *BlockBookClient) doRequest(endpoint, method string, body []byte, query url.Values) (*http.Response, error) {
	requestUrl := i.EndpointURL()
	requestUrl.Path = path.Join(i.EndpointURL().Path, endpoint)
	req, err := http.NewRequest(method, requestUrl.String(), bytes.NewReader(body))
	if query != nil {
		req.URL.RawQuery = query.Encode()
	}
	if err != nil {
		Log.Errorf("creating: %s", err.Error())
		return nil, fmt.Errorf("creating: %s", err.Error())
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := i.HTTPClient.Do(req)
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok && urlErr.Timeout() {
			Log.Errorf("timed out executing: %s", err.Error())
			return nil, clientErr.MakeRetryable(err)
		}
		Log.Errorf("executing: %s", err.Error())
		return nil, fmt.Errorf("executing: %s", err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		errStr := fmt.Sprintf("not ok (%s %s): responded %s", method, requestUrl.String(), resp.Status)
		Log.Errorf(errStr)

		// log body
		if body, err := ioutil.ReadAll(resp.Body); err != nil {
			Log.Warningf("reading body (%s %s): %s", method, requestUrl.String(), err.Error())
		} else {
			if len(body) > 0 {
				Log.Debugf("not ok response body (%s %s):\n\tstring: %s\n\thexencoded: %s", method, requestUrl.String(), string(body), hex.EncodeToString(body))
			}
		}

		// mark 500 errors as fatal
		if resp.StatusCode >= 500 {
			err := fmt.Errorf("wallet server internal error (%s %s)", method, requestUrl.String())
			return nil, clientErr.MakeRetryable(clientErr.MakeFatal(err))
		}
		return nil, fmt.Errorf("status not ok: %s", resp.Status)
	}
	return resp, nil
}

// GetInfo is unused for now so we will not implement it yet
func (i *BlockBookClient) GetInfo() (*model.Info, error) {
	return nil, nil
}

func (i *BlockBookClient) GetTransaction(txid string) (*model.Transaction, error) {
	type resIn struct {
		model.Input
		Addresses []string `json:"addresses"`
	}
	type resOut struct {
		model.Output
		Spent bool `json:"spent"`
	}
	type resTx struct {
		model.Transaction
		Hex  string   `json:"hex"`
		Vin  []resIn  `json:"vin"`
		Vout []resOut `json:"vout"`
	}
	resp, err := i.RequestFunc("tx/"+txid, http.MethodGet, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	tx := new(resTx)
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(tx); err != nil {
		return nil, fmt.Errorf("error decoding transactions: %s", err)
	}
	for n, in := range tx.Vin {
		f, err := model.ToFloat(in.ValueIface)
		if err != nil {
			return nil, err
		}
		tx.Vin[n].Value = f
	}
	for n, out := range tx.Vout {
		f, err := model.ToFloat(out.ValueIface)
		if err != nil {
			return nil, err
		}
		tx.Vout[n].Value = f
	}
	raw, err := hex.DecodeString(tx.Hex)
	if err != nil {
		return nil, err
	}
	ctx := model.Transaction{
		BlockHash:     tx.BlockHash,
		BlockHeight:   tx.BlockHeight,
		BlockTime:     tx.BlockTime,
		Confirmations: tx.Confirmations,
		Locktime:      tx.Locktime,
		RawBytes:      raw,
		Time:          tx.Time,
		Txid:          tx.Txid,
		Version:       tx.Version,
	}
	for n, i := range tx.Vin {
		newIn := model.Input{
			Addr:      i.Addr,
			N:         i.N,
			Satoshis:  i.Satoshis,
			ScriptSig: i.ScriptSig,
			Sequence:  i.Sequence,
			Txid:      i.Txid,
			Value:     tx.Vin[n].Value,
			Vout:      i.Vout,
		}
		if len(i.Addresses) > 0 {
			newIn.Addr = maybeTrimCashAddrPrefix(i.Addresses[0])
		}
		ctx.Inputs = append(ctx.Inputs, newIn)
	}
	for n, o := range tx.Vout {
		newOut := model.Output{
			Value:        tx.Vout[n].Value,
			N:            o.N,
			ScriptPubKey: o.ScriptPubKey,
		}
		for i, addr := range newOut.ScriptPubKey.Addresses {
			newOut.ScriptPubKey.Addresses[i] = maybeTrimCashAddrPrefix(addr)
		}
		ctx.Outputs = append(ctx.Outputs, newOut)
	}
	return &ctx, nil
}

// GetRawTransaction is unused for now so we will not implement it yet
func (i *BlockBookClient) GetRawTransaction(txid string) ([]byte, error) {
	return nil, nil
}

// GetTransactions returns the transactions for a given address. If a single address
// query fails this method will not return an error. Instead it will log the error
// and returns the transactions for the other addresses.
func (i *BlockBookClient) GetTransactions(addrs []btcutil.Address) ([]model.Transaction, error) {
	var txs []model.Transaction
	type txsOrError struct {
		Txs []model.Transaction
		Err error
	}
	var (
		txChan    = make(chan txsOrError)
		queryChan = make(chan struct{}, maxInfightQueries)
		wg        sync.WaitGroup
	)
	wg.Add(len(addrs))
	go func() {
		for _, addr := range addrs {
			queryChan <- struct{}{}
			go func(a btcutil.Address) {
				txs, err := i.getTransactions(maybeConvertCashAddress(a))
				txChan <- txsOrError{txs, err}
				<-queryChan
				wg.Done()
			}(addr)
		}
		wg.Wait()
		close(txChan)
	}()
	for toe := range txChan {
		if toe.Err != nil {
			Log.Errorf("Error querying address from blockbook: %s", toe.Err.Error())
			return nil, toe.Err
		}
		txs = append(txs, toe.Txs...)
	}
	return txs, nil
}

func (i *BlockBookClient) getTransactions(addr string) ([]model.Transaction, error) {
	var ret []model.Transaction
	type resAddr struct {
		TotalPages   int      `json:"totalPages"`
		Transactions []string `json:"transactions"`
	}
	type txOrError struct {
		Tx  *model.Transaction
		Err error
	}
	page := 1
	for {
		q, err := url.ParseQuery("?page=" + strconv.Itoa(page))
		if err != nil {
			return nil, err
		}
		resp, err := i.RequestFunc("/address/"+addr, http.MethodGet, nil, q)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		res := new(resAddr)
		decoder := json.NewDecoder(resp.Body)
		if err = decoder.Decode(res); err != nil {
			return nil, fmt.Errorf("error decoding addrs response: %s", err)
		}
		txChan := make(chan txOrError)
		go func() {
			var wg sync.WaitGroup
			wg.Add(len(res.Transactions))
			for _, txid := range res.Transactions {
				go func(id string) {
					tx, err := i.GetTransaction(id)
					txChan <- txOrError{tx, err}
					wg.Done()
				}(txid)
			}
			wg.Wait()
			close(txChan)
		}()
		for toe := range txChan {
			if toe.Err != nil {
				return nil, err
			}
			if toe.Tx != nil {
				ret = append(ret, *toe.Tx)
			}
		}
		if res.TotalPages <= page {
			break
		}
		page++
	}
	return ret, nil
}

// GetUtxos returns the utxos for a given address. If a single address
// query fails this method will not return an error. Instead it will log the error
// and returns the transactions for the other addresses.
func (i *BlockBookClient) GetUtxos(addrs []btcutil.Address) ([]model.Utxo, error) {
	var ret []model.Utxo
	type utxoOrError struct {
		Utxo *model.Utxo
		Err  error
	}
	var (
		wg        sync.WaitGroup
		queryChan = make(chan struct{}, maxInfightQueries)
		utxoChan  = make(chan utxoOrError)
	)
	wg.Add(len(addrs))
	go func() {
		for _, addr := range addrs {
			queryChan <- struct{}{}
			go func(addr btcutil.Address) {
				defer wg.Done()
				defer func() {
					<-queryChan
				}()
				resp, err := i.RequestFunc("/utxo/"+maybeConvertCashAddress(addr), http.MethodGet, nil, nil)
				if err != nil {
					utxoChan <- utxoOrError{nil, err}
					return
				}
				defer resp.Body.Close()
				var utxos []model.Utxo
				decoder := json.NewDecoder(resp.Body)
				if err = decoder.Decode(&utxos); err != nil {
					utxoChan <- utxoOrError{nil, err}
					return
				}
				for z, u := range utxos {
					f, err := model.ToFloat(u.AmountIface)
					if err != nil {
						utxoChan <- utxoOrError{nil, err}
						return
					}
					utxos[z].Amount = f
				}
				var wg2 sync.WaitGroup
				wg2.Add(len(utxos))
				for _, u := range utxos {
					go func(ut model.Utxo) {
						defer wg2.Done()

						tx, err := i.GetTransaction(ut.Txid)
						if err != nil {
							utxoChan <- utxoOrError{nil, err}
							return
						}
						if len(tx.Outputs)-1 < ut.Vout {
							utxoChan <- utxoOrError{nil, errors.New("transaction has invalid number of outputs")}
							return
						}
						ut.ScriptPubKey = tx.Outputs[ut.Vout].ScriptPubKey.Hex
						if len(tx.Outputs[ut.Vout].ScriptPubKey.Addresses[0]) > 0 {
							ut.Address = maybeTrimCashAddrPrefix(tx.Outputs[ut.Vout].ScriptPubKey.Addresses[0])
						}
						utxoChan <- utxoOrError{&ut, nil}
					}(u)
				}
				wg2.Wait()
			}(addr)
		}
		wg.Wait()
		close(utxoChan)
	}()
	for toe := range utxoChan {
		if toe.Err != nil {
			Log.Errorf("Error querying utxos from blockbook: %s", toe.Err.Error())
			return nil, toe.Err
		}
		if toe.Utxo != nil {
			ret = append(ret, *toe.Utxo)
		}
	}
	return ret, nil
}

func (i *BlockBookClient) BlockNotify() <-chan model.Block {
	return i.blockNotifyChan
}

func (i *BlockBookClient) TransactionNotify() <-chan model.Transaction {
	return i.txNotifyChan
}

func (i *BlockBookClient) ListenAddresses(addrs ...btcutil.Address) {
	if len(addrs) == 0 {
		return
	}

	i.listenLock.Lock()
	defer i.listenLock.Unlock()
	i.socketMutex.RLock()
	defer i.socketMutex.RUnlock()

	var convertedAddrs []string
	for _, addr := range addrs {
		convertedAddrs = append(convertedAddrs, maybeConvertCashAddress(addr))
	}

	if i.SocketClient != nil {
		i.SocketClient.Emit("subscribe", []interface{}{"bitcoind/addresstxid", convertedAddrs})
	} else {
		i.listenQueue = append(i.listenQueue, convertedAddrs...)
	}
}

func connectSocket(u *url.URL, proxyDialer proxy.Dialer) (model.SocketClient, error) {
	socketClient, err := gosocketio.Dial(
		gosocketio.GetUrl(u.Hostname(), model.DefaultPort(u), model.HasImpliedURLSecurity(u)),
		transport.GetDefaultWebsocketTransport(proxyDialer),
	)
	if err != nil {
		return nil, err
	}

	// Signal readyness on connection
	socketReady := make(chan struct{})
	socketClient.On(gosocketio.OnConnection, func(h *gosocketio.Channel, args interface{}) {
		close(socketReady)
	})

	// Wait for socket to be ready or timeout
	select {
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("websocket connection timeout (%s)", u.Host)
	case <-socketReady:
		break
	}
	return socketClient, nil
}

func (i *BlockBookClient) setupListeners() error {
	i.listenLock.Lock()
	defer i.listenLock.Unlock()

	if i.SocketClient != nil {
		return nil
	}

	client, err := connectSocket(i.apiUrl, i.proxyDialer)
	if err != nil {
		Log.Errorf("reconnect websocket (%s): %s", i.String(), err.Error())
		var (
			setupTimeoutAt = time.Now().Add(10 * time.Second)
			t              = time.NewTicker(2 * time.Second)
		)
		defer t.Stop()
		for range t.C {
			if time.Now().After(setupTimeoutAt) {
				return fmt.Errorf("websocket reconnection timeout (%s)", i.String())
			}
			client, err = connectSocket(i.apiUrl, i.proxyDialer)
			if err != nil {
				Log.Errorf("reconnect websocket (%s): %s", i.String(), err.Error())
				continue
			}
			break
		}
	}
	i.SocketClient = client
	go i.websocketWatchdog.guardWebsocket()

	i.SocketClient.On(gosocketio.OnError, func(c *gosocketio.Channel, args interface{}) {
		Log.Warningf("websocket error (%s): %+v", i.String(), "-", args)
		i.websocketWatchdog.bark()
	})
	i.SocketClient.On(gosocketio.OnDisconnection, func(c *gosocketio.Channel) {
		Log.Warningf("websocket disconnected (%s)", i.String())
		i.websocketWatchdog.bark()
	})

	i.SocketClient.On("bitcoind/hashblock", func(h *gosocketio.Channel, arg interface{}) {
		best, err := i.GetBestBlock()
		if err != nil {
			Log.Errorf("error downloading best block: %s", err.Error())
			return
		}
		i.blockNotifyChan <- *best
	})
	i.SocketClient.Emit("subscribe", protocol.ToArgArray("bitcoind/hashblock"))

	i.SocketClient.On("bitcoind/addresstxid", func(h *gosocketio.Channel, arg interface{}) {
		m, ok := arg.(map[string]interface{})
		if !ok {
			Log.Errorf("error checking type after socket notification: %T", arg)
			return
		}
		for _, v := range m {
			txid, ok := v.(string)
			if !ok {
				Log.Errorf("error checking type after socket notification: %T", arg)
				return
			}
			_, err := chainhash.NewHashFromStr(txid) // Check is 256 bit hash. Might also be address
			if err == nil {
				tx, err := i.GetTransaction(txid)
				if err != nil {
					Log.Errorf("error downloading tx after socket notification: %s", err.Error())
					return
				}
				tx.Time = time.Now().Unix()
				i.txNotifyChan <- *tx
			}
		}
	})

	// Subscribe to queued addresses
	if len(i.listenQueue) != 0 {
		i.SocketClient.Emit("subscribe", []interface{}{"bitcoind/addresstxid", i.listenQueue})
		i.listenQueue = []string{}
	}

	Log.Infof("websocket connected (%s)", i.String())
	return nil
}

func (i *BlockBookClient) Broadcast(tx []byte) (string, error) {
	txHex := hex.EncodeToString(tx)
	resp, err := i.RequestFunc("sendtx/"+txHex, http.MethodGet, nil, nil)
	if err != nil {
		return "", fmt.Errorf("error broadcasting tx: %s", err)
	}
	defer resp.Body.Close()

	type Response struct {
		Txid string `json:"result"`
	}
	rs := new(Response)
	if err = json.NewDecoder(resp.Body).Decode(rs); err != nil {
		return "", fmt.Errorf("error decoding txid: %s", err)
	}
	return rs.Txid, nil
}

func (i *BlockBookClient) GetBestBlock() (*model.Block, error) {
	type backend struct {
		Blocks        int    `json:"blocks"`
		BestBlockHash string `json:"bestBlockHash"`
	}
	type resIndex struct {
		Backend backend `json:"backend"`
	}

	type resBlockHash struct {
		BlockHash string `json:"blockHash"`
	}

	resp, err := i.RequestFunc("", http.MethodGet, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("getting block index: %s", err.Error())
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	bi := new(resIndex)
	if err = decoder.Decode(bi); err != nil {
		return nil, fmt.Errorf("decoding block index: %s", err)
	}
	blockIndexPath := "/block-index/" + strconv.Itoa(bi.Backend.Blocks-1)

	resp2, err := i.RequestFunc(blockIndexPath, http.MethodGet, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("getting block detail (%s): %s", blockIndexPath, err.Error())
	}
	defer resp2.Body.Close()

	decoder2 := json.NewDecoder(resp2.Body)
	bh := new(resBlockHash)
	if err = decoder2.Decode(bh); err != nil {
		return nil, fmt.Errorf("decoding block detail: %s", err)
	}

	return &model.Block{
		Hash:              bi.Backend.BestBlockHash,
		Height:            bi.Backend.Blocks,
		PreviousBlockhash: bh.BlockHash,
	}, nil
}

func (i *BlockBookClient) GetBlocksBefore(to time.Time, limit int) (*model.BlockList, error) {
	resp, err := i.RequestFunc("blocks", http.MethodGet, nil, url.Values{
		"blockDate":      {to.Format("2006-01-02")},
		"startTimestamp": {fmt.Sprint(to.Unix())},
		"limit":          {fmt.Sprint(limit)},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	list := new(model.BlockList)
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(list); err != nil {
		return nil, fmt.Errorf("error decoding block list: %s", err)
	}
	return list, nil
}

func (i *BlockBookClient) EstimateFee(nbBlocks int) (int, error) {
	resp, err := i.RequestFunc("utils/estimatefee", http.MethodGet, nil, url.Values{"nbBlocks": {fmt.Sprint(nbBlocks)}})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data := map[int]float64{}
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, fmt.Errorf("error decoding fee estimate: %s", err)
	}
	return int(data[nbBlocks] * 1e8), nil
}

// maybeConvertCashAddress adds the Bitcoin Cash URI prefix to the address
// to make blockbook happy if this is a cashaddr.
func maybeConvertCashAddress(addr btcutil.Address) string {
	_, isP2PKHCashAddr := addr.(*bchutil.CashAddressPubKeyHash)
	_, isP2SHCashAddr := addr.(*bchutil.CashAddressScriptHash)
	if isP2PKHCashAddr || isP2SHCashAddr {
		if addr.IsForNet(&chaincfg.MainNetParams) {
			return "bitcoincash:" + addr.String()
		} else {
			return "bchtest:" + addr.String()
		}
	}
	return addr.String()
}

func maybeTrimCashAddrPrefix(addr string) string {
	return strings.TrimPrefix(strings.TrimPrefix(addr, "bchtest:"), "bitcoincash:")
}
