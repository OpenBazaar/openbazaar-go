package insight

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	gosocketio "github.com/OpenBazaar/golang-socketio"
	"github.com/OpenBazaar/golang-socketio/protocol"
	"github.com/OpenBazaar/multiwallet/client/transport"
	"github.com/OpenBazaar/multiwallet/model"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
	"github.com/op/go-logging"
	"golang.org/x/net/proxy"
)

var Log = logging.MustGetLogger("client")

type InsightClient struct {
	apiUrl          url.URL
	blockNotifyChan chan model.Block
	txNotifyChan    chan model.Transaction
	proxyDialer     proxy.Dialer

	listenQueue []string
	listenLock  sync.Mutex

	HTTPClient   http.Client
	RequestFunc  func(endpoint, method string, body []byte, query url.Values) (*http.Response, error)
	SocketClient model.SocketClient
}

func NewInsightClient(apiUrl string, proxyDialer proxy.Dialer) (*InsightClient, error) {
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

	bch := make(chan model.Block)
	tch := make(chan model.Transaction)
	tbTransport := &http.Transport{Dial: dial}
	ic := &InsightClient{
		HTTPClient:      http.Client{Timeout: time.Second * 30, Transport: tbTransport},
		apiUrl:          *u,
		proxyDialer:     proxyDialer,
		blockNotifyChan: bch,
		txNotifyChan:    tch,
		listenLock:      sync.Mutex{},
	}
	ic.RequestFunc = ic.doRequest
	return ic, nil
}

func (i *InsightClient) BlockChannel() chan model.Block {
	return i.blockNotifyChan
}

func (i *InsightClient) TxChannel() chan model.Transaction {
	return i.txNotifyChan
}

func (i *InsightClient) Start() error {
	return i.setupListeners(i.apiUrl, i.proxyDialer)
}

func (i *InsightClient) Close() {
	if i.SocketClient != nil {
		i.SocketClient.Close()
	}
}

func validateScheme(target *url.URL) error {
	switch target.Scheme {
	case "https", "http":
		return nil
	}
	return fmt.Errorf("unsupported scheme: %s", target.Scheme)
}

func (i *InsightClient) doRequest(endpoint, method string, body []byte, query url.Values) (*http.Response, error) {
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

	resp, err := i.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	// Try again if for some reason it returned a bad request
	if resp.StatusCode == http.StatusBadRequest {
		// Reset the body so we can read it again.
		req.Body = ioutil.NopCloser(bytes.NewReader(body))
		resp, err = i.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status not ok: %s", resp.Status)
	}
	return resp, nil
}

func (i *InsightClient) GetInfo() (*model.Info, error) {
	q, err := url.ParseQuery("q=values")
	if err != nil {
		return nil, err
	}
	resp, err := i.RequestFunc("status", http.MethodGet, nil, q)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(resp.Body)
	stat := new(model.Status)
	defer resp.Body.Close()
	if err = decoder.Decode(stat); err != nil {
		return nil, fmt.Errorf("error decoding status: %s", err)
	}
	info := stat.Info
	f, err := model.ToFloat(stat.Info.RelayFeeIface)
	if err != nil {
		return nil, err
	}
	info.RelayFee = f
	f, err = model.ToFloat(stat.Info.DifficultyIface)
	if err != nil {
		return nil, err
	}
	info.Difficulty = f
	return &info, nil
}

func (i *InsightClient) GetTransaction(txid string) (*model.Transaction, error) {
	resp, err := i.RequestFunc("tx/"+txid, http.MethodGet, nil, nil)
	if err != nil {
		return nil, err
	}
	tx := new(model.Transaction)
	decoder := json.NewDecoder(resp.Body)
	defer resp.Body.Close()
	if err = decoder.Decode(tx); err != nil {
		return nil, fmt.Errorf("error decoding transactions: %s", err)
	}
	for n, in := range tx.Inputs {
		f, err := model.ToFloat(in.ValueIface)
		if err != nil {
			return nil, err
		}
		tx.Inputs[n].Value = f
	}
	for n, out := range tx.Outputs {
		f, err := model.ToFloat(out.ValueIface)
		if err != nil {
			return nil, err
		}
		tx.Outputs[n].Value = f
	}
	return tx, nil
}

func (i *InsightClient) GetRawTransaction(txid string) ([]byte, error) {
	resp, err := i.RequestFunc("rawtx/"+txid, http.MethodGet, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	tx := new(model.RawTxResponse)
	if err = json.NewDecoder(resp.Body).Decode(tx); err != nil {
		return nil, fmt.Errorf("error decoding transactions: %s", err)
	}
	return hex.DecodeString(tx.RawTx)
}

func (i *InsightClient) GetTransactions(addrs []btcutil.Address) ([]model.Transaction, error) {
	var txs []model.Transaction
	from := 0
	for {
		tl, err := i.getTransactions(addrs, from, from+50)
		if err != nil {
			return txs, err
		}
		txs = append(txs, tl.Items...)
		if len(txs) >= tl.TotalItems {
			break
		}
		from += 50
	}
	return txs, nil
}

func (i *InsightClient) getTransactions(addrs []btcutil.Address, from, to int) (*model.TransactionList, error) {
	type req struct {
		Addrs string `json:"addrs"`
		From  int    `json:"from"`
		To    int    `json:"to"`
	}
	s := ``
	for n, addr := range addrs {
		s += addr.String()
		if n < len(addrs)-1 {
			s += ","
		}
	}
	r := &req{
		Addrs: s,
		From:  from,
		To:    to,
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	resp, err := i.RequestFunc("addrs/txs", http.MethodPost, b, nil)
	if err != nil {
		return nil, err
	}
	tl := new(model.TransactionList)
	decoder := json.NewDecoder(resp.Body)
	defer resp.Body.Close()
	if err = decoder.Decode(tl); err != nil {
		return nil, fmt.Errorf("error decoding transaction list: %s", err)
	}
	for z, tx := range tl.Items {
		for n, in := range tx.Inputs {
			f, err := model.ToFloat(in.ValueIface)
			if err != nil {
				return nil, err
			}
			tl.Items[z].Inputs[n].Value = f
		}
		for n, out := range tx.Outputs {
			f, err := model.ToFloat(out.ValueIface)
			if err != nil {
				return nil, err
			}
			tl.Items[z].Outputs[n].Value = f
		}
	}
	return tl, nil
}

func (i *InsightClient) GetUtxos(addrs []btcutil.Address) ([]model.Utxo, error) {
	type req struct {
		Addrs string `json:"addrs"`
	}
	s := ``
	for n, addr := range addrs {
		s += addr.String()
		if n < len(addrs)-1 {
			s += ","
		}
	}
	r := &req{
		Addrs: s,
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	resp, err := i.RequestFunc("addrs/utxo", http.MethodPost, b, nil)
	if err != nil {
		return nil, err
	}
	utxos := []model.Utxo{}
	decoder := json.NewDecoder(resp.Body)
	defer resp.Body.Close()
	if err = decoder.Decode(&utxos); err != nil {
		return nil, fmt.Errorf("error decoding utxo list: %s", err)
	}
	for z, u := range utxos {
		f, err := model.ToFloat(u.AmountIface)
		if err != nil {
			return nil, err
		}
		utxos[z].Amount = f
	}
	return utxos, nil
}

func (i *InsightClient) BlockNotify() <-chan model.Block {
	return i.blockNotifyChan
}

func (i *InsightClient) TransactionNotify() <-chan model.Transaction {
	return i.txNotifyChan
}

func (i *InsightClient) ListenAddress(addr btcutil.Address) {
	i.listenLock.Lock()
	defer i.listenLock.Unlock()
	var args []interface{}
	args = append(args, "bitcoind/addresstxid")
	args = append(args, []string{addr.String()})
	if i.SocketClient != nil {
		i.SocketClient.Emit("subscribe", args)
	} else {
		i.listenQueue = append(i.listenQueue, addr.String())
	}
}

func (i *InsightClient) setupListeners(u url.URL, proxyDialer proxy.Dialer) error {
	i.listenLock.Lock()
	defer i.listenLock.Unlock()
	if i.SocketClient == nil {
		socketClient, err := gosocketio.Dial(
			gosocketio.GetUrl(u.Hostname(), model.DefaultPort(u), model.HasImpliedURLSecurity(u)),
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
				return errors.New("websocket timed out")
			case <-socketReady:
				break
			}
			i.SocketClient = socketClient
		} else {
			return err
		}
	}

	i.SocketClient.On("bitcoind/hashblock", func(h *gosocketio.Channel, arg interface{}) {
		best, err := i.GetBestBlock()
		if err != nil {
			Log.Errorf("Error downloading best block: %s", err.Error())
			return
		}
		i.blockNotifyChan <- *best
	})
	i.SocketClient.Emit("subscribe", protocol.ToArgArray("bitcoind/hashblock"))

	i.SocketClient.On("bitcoind/addresstxid", func(h *gosocketio.Channel, arg interface{}) {
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
		i.SocketClient.Emit("subscribe", args)
	}
	i.listenQueue = []string{}
	Log.Infof("Connected to websocket endpoint %s", u.Host)
	return nil
}

func (i *InsightClient) Broadcast(tx []byte) (string, error) {
	txHex := hex.EncodeToString(tx)
	type RawTx struct {
		Raw string `json:"rawtx"`
	}
	t := RawTx{txHex}
	txJson, err := json.Marshal(&t)
	if err != nil {
		return "", fmt.Errorf("error encoding tx: %s", err)
	}
	resp, err := i.RequestFunc("tx/send", http.MethodPost, txJson, nil)
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

func (i *InsightClient) GetBestBlock() (*model.Block, error) {
	q, err := url.ParseQuery("limit=2")
	if err != nil {
		return nil, err
	}
	resp, err := i.RequestFunc("blocks", http.MethodGet, nil, q)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(resp.Body)
	sl := new(model.BlockList)
	defer resp.Body.Close()
	if err = decoder.Decode(sl); err != nil {
		return nil, fmt.Errorf("error decoding block list: %s", err)
	}
	if len(sl.Blocks) < 2 {
		return nil, fmt.Errorf("API returned incorrect number of block summaries: n=%d", len(sl.Blocks))
	}
	sum := sl.Blocks[0]
	sum.PreviousBlockhash = sl.Blocks[1].Hash
	return &sum, nil
}

func (i *InsightClient) GetBlocksBefore(to time.Time, limit int) (*model.BlockList, error) {
	resp, err := i.RequestFunc("blocks", http.MethodGet, nil, url.Values{
		"blockDate":      {to.Format("2006-01-02")},
		"startTimestamp": {fmt.Sprint(to.Unix())},
		"limit":          {fmt.Sprint(limit)},
	})
	if err != nil {
		return nil, err
	}
	list := new(model.BlockList)
	decoder := json.NewDecoder(resp.Body)
	defer resp.Body.Close()
	if err = decoder.Decode(list); err != nil {
		return nil, fmt.Errorf("error decoding block list: %s", err)
	}
	return list, nil
}

func (i *InsightClient) EstimateFee(nbBlocks int) (int, error) {
	resp, err := i.RequestFunc("utils/estimatefee", http.MethodGet, nil, url.Values{"nbBlocks": {fmt.Sprint(nbBlocks)}})
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
