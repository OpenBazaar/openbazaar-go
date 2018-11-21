package blockbook

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/OpenBazaar/multiwallet/client"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/OpenBazaar/golang-socketio"
	"github.com/OpenBazaar/golang-socketio/protocol"
	"github.com/OpenBazaar/multiwallet/client/transport"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
	"github.com/op/go-logging"
	"golang.org/x/net/proxy"
)

var Log = logging.MustGetLogger("client")

type BlockBookClient struct {
	httpClient      http.Client
	apiUrl          url.URL
	blockNotifyChan chan client.Block
	txNotifyChan    chan client.Transaction
	socketClient    client.SocketClient
	proxyDialer     proxy.Dialer

	listenQueue []string
	listenLock  sync.Mutex
}

func NewBlockBookClient(apiUrl string, proxyDialer proxy.Dialer) (*BlockBookClient, error) {
	u, err := url.Parse(apiUrl)
	if err != nil {
		return nil, err
	}

	if err := validateScheme(u); err != nil {
		return nil, err
	}

	dial := net.Dial
	if proxyDialer != nil {
		dial = proxyDialer.Dial
	}

	bch := make(chan client.Block)
	tch := make(chan client.Transaction)
	tbTransport := &http.Transport{Dial: dial}
	ic := &BlockBookClient{
		httpClient:      http.Client{Timeout: time.Second * 30, Transport: tbTransport},
		apiUrl:          *u,
		proxyDialer:     proxyDialer,
		blockNotifyChan: bch,
		txNotifyChan:    tch,
		listenLock:      sync.Mutex{},
	}
	return ic, nil
}

func (i *BlockBookClient) Start() {
	go i.setupListeners(i.apiUrl, i.proxyDialer)
}

func (i *BlockBookClient) Close() {
	if i.socketClient != nil {
		i.socketClient.Close()
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
	requestUrl := i.apiUrl
	requestUrl.Path = path.Join(i.apiUrl.Path, endpoint)
	req, err := http.NewRequest(method, requestUrl.String(), bytes.NewReader(body))
	if query != nil {
		req.URL.RawQuery = query.Encode()
	}
	if err != nil {
		return nil, fmt.Errorf("creating request: %s", err)
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	// Try again if for some reason it returned a bad request
	if resp.StatusCode == http.StatusBadRequest {
		// Reset the body so we can read it again.
		req.Body = ioutil.NopCloser(bytes.NewReader(body))
		resp, err = i.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status not ok: %s", resp.Status)
	}
	return resp, nil
}

// GetInfo is unused for now so we will not implement it yet
func (i *BlockBookClient) GetInfo() (*client.Info, error) {
	return nil, nil
}

func (i *BlockBookClient) GetTransaction(txid string) (*client.Transaction, error) {
	type resIn struct {
		client.Input
		Addresses []string `json:"addresses"`
	}
	type resOut struct {
		client.Output
		Spent bool `json:"spent"`
	}
	type resTx struct {
		client.Transaction
		Hex string `json:"hex"`
		Vin []resIn `json:"vin"`
		Vout []resOut `json:"vout"`
	}
	resp, err := i.doRequest("tx/"+txid, http.MethodGet, nil, nil)
	if err != nil {
		return nil, err
	}
	tx := new(resTx)
	decoder := json.NewDecoder(resp.Body)
	defer resp.Body.Close()
	if err = decoder.Decode(tx); err != nil {
		return nil, fmt.Errorf("error decoding transactions: %s", err)
	}
	for n, in := range tx.Vin {
		f, err := toFloat(in.ValueIface)
		if err != nil {
			return nil, err
		}
		tx.Vin[n].Value = f
	}
	for n, out := range tx.Vout {
		f, err := toFloat(out.ValueIface)
		if err != nil {
			return nil, err
		}
		tx.Vout[n].Value = f
	}
	raw, err := hex.DecodeString(tx.Hex)
	if err != nil {
		return nil, err
	}
	ctx := client.Transaction{
		Txid: tx.Txid,
		Confirmations: tx.Confirmations,
		RawBytes: raw,
		BlockHeight: tx.BlockHeight,
		BlockTime: tx.BlockTime,
		BlockHash: tx.BlockHash,
		Time: tx.Time,
		Version: tx.Version,
	}
	for n, i := range tx.Vin {
		newIn := client.Input{
			Txid: i.Txid,
			Value: tx.Vin[n].Value,
			N: i.N,
			Sequence: i.Sequence,
			ScriptSig: i.ScriptSig,
		}
		if len(i.Addresses) > 0 {
			newIn.Addr = i.Addresses[0]
		}
		ctx.Inputs = append(ctx.Inputs, newIn)
	}
	for n, o := range tx.Vout {
		newOut := client.Output {
			Value: tx.Vout[n].Value,
			N: o.N,
			ScriptPubKey: o.ScriptPubKey,
		}
		ctx.Outputs = append(ctx.Outputs, newOut)
	}
	return &ctx, nil
}

// GetRawTransaction is unused for now so we will not implement it yet
func (i *BlockBookClient) GetRawTransaction(txid string) ([]byte, error) {
	return nil, nil
}

func (i *BlockBookClient) GetTransactions(addrs []btcutil.Address) ([]client.Transaction, error) {
	var txs []client.Transaction
	type txsOrError struct {
		Txs []client.Transaction
		Err error
	}
	txChan := make(chan txsOrError)
	go func() {
		var wg sync.WaitGroup
		wg.Add(len(addrs))
		for _, addr := range addrs {
			go func(a string) {
				txs, err := i.getTransactions(a)
				txChan <- txsOrError{txs, err}
				wg.Done()
			}(addr.String())
		}
		wg.Wait()
		close(txChan)
	}()
	for toe := range txChan {
		if toe.Err != nil {
			return nil, toe.Err
		}
		txs = append(txs, toe.Txs...)
	}
	return txs, nil
}

func (i *BlockBookClient) getTransactions(addr string) ([]client.Transaction, error) {
	var ret []client.Transaction
	type resAddr struct {
		TotalPages int `json:"totalPages"`
		Transactions []string `json:"transactions"`
	}
	type txOrError struct {
		Tx client.Transaction
		Err error
	}
	page := 1
	for {
		q, err := url.ParseQuery("?page=" + strconv.Itoa(page))
		if err != nil {
			return nil, err
		}
		resp, err := i.doRequest("/address/"+addr, http.MethodGet, nil, q)
		if err != nil {
			return nil, err
		}
		res := new(resAddr)
		decoder := json.NewDecoder(resp.Body)
		defer resp.Body.Close()
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
					txChan <- txOrError{*tx, err}
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
			ret = append(ret, toe.Tx)
		}
		if res.TotalPages <= page {
			break
		}
		page++
	}
	return ret, nil
}

func (i *BlockBookClient) GetUtxos(addrs []btcutil.Address) ([]client.Utxo, error) {
	var ret []client.Utxo
	type utxoOrError struct {
		Utxo *client.Utxo
		Err error
	}
	utxoChan := make(chan utxoOrError)
	var wg sync.WaitGroup
	go func() {
		wg.Add(len(addrs))
		for _, addr := range addrs {
			go func(addr btcutil.Address) {
				defer wg.Done()

				resp, err := i.doRequest("/utxo/"+addr.String(), http.MethodGet, nil, nil)
				if err != nil {
					utxoChan <- utxoOrError{nil, err}
					return
				}
				var utxos []client.Utxo
				decoder := json.NewDecoder(resp.Body)
				defer resp.Body.Close()
				if err = decoder.Decode(&utxos); err != nil {
					utxoChan <- utxoOrError{nil, err}
					return
				}
				for z, u := range utxos {
					f, err := toFloat(u.AmountIface)
					if err != nil {
						utxoChan <- utxoOrError{nil, err}
						return
					}
					utxos[z].Amount = f
				}
				var wg2 sync.WaitGroup
				wg2.Add(len(utxos))
				for _, u := range utxos {
					go func(ut client.Utxo) {
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
							ut.Address = tx.Outputs[ut.Vout].ScriptPubKey.Addresses[0]
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
			return nil, toe.Err
		}
		ret = append(ret, *toe.Utxo)
	}
	return ret, nil
}

func (i *BlockBookClient) BlockNotify() <-chan client.Block {
	return i.blockNotifyChan
}

func (i *BlockBookClient) TransactionNotify() <-chan client.Transaction {
	return i.txNotifyChan
}

func (i *BlockBookClient) ListenAddress(addr btcutil.Address) {
	i.listenLock.Lock()
	defer i.listenLock.Unlock()
	var args []interface{}
	args = append(args, "bitcoind/addresstxid")
	args = append(args, []string{addr.String()})
	if i.socketClient != nil {
		i.socketClient.Emit("subscribe", args)
	} else {
		i.listenQueue = append(i.listenQueue, addr.String())
	}
}

func (i *BlockBookClient) setupListeners(u url.URL, proxyDialer proxy.Dialer) {
	for {
		if i.socketClient != nil {
			i.listenLock.Lock()
			break
		}
		socketClient, err := gosocketio.Dial(
			gosocketio.GetUrl(u.Hostname(), defaultPort(u), hasImpliedURLSecurity(u)),
			transport.GetDefaultWebsocketTransport(proxyDialer),
		)
		if err == nil {
			socketReady := make(chan struct{})
			socketClient.On(gosocketio.OnConnection, func(h *gosocketio.Channel, args interface{}) {
				close(socketReady)
			})
			select {
			case <-time.After(10 * time.Second):
				Log.Warningf("Timeout connecting to websocket endpoint %s", u.Host)
				continue
			case <-socketReady:
				break
			}
			i.socketClient = socketClient
			continue
		}
		if time.Now().Unix()%60 == 0 {
			Log.Warningf("Failed to connect to websocket endpoint %s", u.Host)
		}
		time.Sleep(time.Second * 2)
	}

	i.socketClient.On("bitcoind/hashblock", func(h *gosocketio.Channel, arg interface{}) {
		best, err := i.GetBestBlock()
		if err != nil {
			Log.Errorf("Error downloading best block: %s", err.Error())
			return
		}
		i.blockNotifyChan <- *best
	})
	i.socketClient.Emit("subscribe", protocol.ToArgArray("bitcoind/hashblock"))

	i.socketClient.On("bitcoind/addresstxid", func(h *gosocketio.Channel, arg interface{}) {
		m, ok := arg.(map[string]interface{})
		if !ok {
			Log.Errorf("Error checking type after socket notification: %T", arg)
			return
		}
		for _, v := range m {
			txid, ok := v.(string)
			if !ok {
				Log.Errorf("Error checking type after socket notification: %T", arg)
				return
			}
			_, err := chainhash.NewHashFromStr(txid) // Check is 256 bit hash. Might also be address
			if err == nil {
				tx, err := i.GetTransaction(txid)
				if err != nil {
					Log.Errorf("Error downloading tx after socket notification: %s", err.Error())
					return
				}
				i.txNotifyChan <- *tx
			}
		}
	})
	for _, addr := range i.listenQueue {
		var args []interface{}
		args = append(args, "bitcoind/addresstxid")
		args = append(args, []string{addr})
		i.socketClient.Emit("subscribe", args)
	}
	i.listenQueue = []string{}
	i.listenLock.Unlock()
	Log.Infof("Connected to websocket endpoint %s", u.Host)
}

func defaultPort(u url.URL) int {
	var port int
	if parsedPort, err := strconv.ParseInt(u.Port(), 10, 32); err == nil {
		port = int(parsedPort)
	}
	if port == 0 {
		if hasImpliedURLSecurity(u) {
			port = 443
		} else {
			port = 80
		}
	}
	return port
}

func hasImpliedURLSecurity(u url.URL) bool { return u.Scheme == "https" }

func (i *BlockBookClient) Broadcast(tx []byte) (string, error) {
	txHex := hex.EncodeToString(tx)
	resp, err := i.doRequest("sendtx/"+txHex, http.MethodGet, nil, nil)
	if err != nil {
		return "", fmt.Errorf("error broadcasting tx: %s", err)
	}
	defer resp.Body.Close()

	type Response struct {
		Txid string `json:"txid"`
	}
	rs := new(Response)
	if err = json.NewDecoder(resp.Body).Decode(rs); err != nil {
		return "", fmt.Errorf("error decoding txid: %s", err)
	}
	return rs.Txid, nil
}

func (i *BlockBookClient) GetBestBlock() (*client.Block, error) {
	type backend struct {
		Blocks int `json:"blocks"`
		BestBlockHash string `json:"bestBlockHash"`
	}
	type resIndex struct {
		Backend backend `json:"backend"`
	}

	type resBlockHash struct {
		BlockHash string `json:"blockHash"`
	}

	resp, err := i.doRequest("", http.MethodGet, nil, nil)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(resp.Body)
	bi := new(resIndex)
	defer resp.Body.Close()
	if err = decoder.Decode(bi); err != nil {
		return nil, fmt.Errorf("error decoding block index: %s", err)
	}
	resp2, err := i.doRequest("/block-index/"+strconv.Itoa(bi.Backend.Blocks-1), http.MethodGet, nil, nil)
	if err != nil {
		return nil, err
	}
	decoder2 := json.NewDecoder(resp2.Body)
	bh := new(resBlockHash)
	defer resp2.Body.Close()
	if err = decoder2.Decode(bh); err != nil {
		return nil, fmt.Errorf("error decoding block hash: %s", err)
	}
	ret := client.Block{
		Hash: bi.Backend.BestBlockHash,
		Height: bi.Backend.Blocks,
		PreviousBlockhash: bh.BlockHash,
	}
	return &ret, nil
}

func (i *BlockBookClient) GetBlocksBefore(to time.Time, limit int) (*client.BlockList, error) {
	resp, err := i.doRequest("blocks", http.MethodGet, nil, url.Values{
		"blockDate":      {to.Format("2006-01-02")},
		"startTimestamp": {fmt.Sprint(to.Unix())},
		"limit":          {fmt.Sprint(limit)},
	})
	if err != nil {
		return nil, err
	}
	list := new(client.BlockList)
	decoder := json.NewDecoder(resp.Body)
	defer resp.Body.Close()
	if err = decoder.Decode(list); err != nil {
		return nil, fmt.Errorf("error decoding block list: %s", err)
	}
	return list, nil
}

// API sometimees returns a float64 or a string so we'll always convert it into a float64
func toFloat(i interface{}) (float64, error) {
	_, fok := i.(float64)
	_, sok := i.(string)
	if fok {
		return i.(float64), nil
	} else if sok {
		s := i.(string)
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("error parsing value float: %s", err)
		}
		return f, nil
	} else {
		return 0, errors.New("Unknown value type in response")
	}
}

func (i *BlockBookClient) EstimateFee(nbBlocks int) (int, error) {
	resp, err := i.doRequest("utils/estimatefee", http.MethodGet, nil, url.Values{"nbBlocks": {fmt.Sprint(nbBlocks)}})
	if err != nil {
		return 0, err
	}
	data := map[int]float64{}
	defer resp.Body.Close()
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, fmt.Errorf("error decoding fee estimate: %s", err)
	}
	return int(data[nbBlocks] * 1e8), nil
}
