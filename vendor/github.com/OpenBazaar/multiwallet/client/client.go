package client

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

type InsightClient struct {
	httpClient      http.Client
	apiUrl          url.URL
	blockNotifyChan chan Block
	txNotifyChan    chan Transaction
	socketClient    SocketClient
	proxyDialer     proxy.Dialer

	listenQueue []string
	listenLock  sync.Mutex
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

	bch := make(chan Block)
	tch := make(chan Transaction)
	tbTransport := &http.Transport{Dial: dial}
	ic := &InsightClient{
		httpClient:      http.Client{Timeout: time.Second * 30, Transport: tbTransport},
		apiUrl:          *u,
		proxyDialer:     proxyDialer,
		blockNotifyChan: bch,
		txNotifyChan:    tch,
		listenLock:      sync.Mutex{},
	}
	return ic, nil
}

func (i *InsightClient) Start() {
	go i.setupListeners(i.apiUrl, i.proxyDialer)
}

func (i *InsightClient) Close() {
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

func (i *InsightClient) GetInfo() (*Info, error) {
	q, err := url.ParseQuery("?q=values")
	if err != nil {
		return nil, err
	}
	resp, err := i.doRequest("status", http.MethodGet, nil, q)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(resp.Body)
	stat := new(Status)
	defer resp.Body.Close()
	if err = decoder.Decode(stat); err != nil {
		return nil, fmt.Errorf("error decoding status: %s", err)
	}
	info := stat.Info
	f, err := toFloat(stat.Info.RelayFeeIface)
	if err != nil {
		return nil, err
	}
	info.RelayFee = f
	f, err = toFloat(stat.Info.DifficultyIface)
	if err != nil {
		return nil, err
	}
	info.Difficulty = f
	return &info, nil
}

func (i *InsightClient) GetTransaction(txid string) (*Transaction, error) {
	resp, err := i.doRequest("tx/"+txid, http.MethodGet, nil, nil)
	if err != nil {
		return nil, err
	}
	tx := new(Transaction)
	decoder := json.NewDecoder(resp.Body)
	defer resp.Body.Close()
	if err = decoder.Decode(tx); err != nil {
		return nil, fmt.Errorf("error decoding transactions: %s", err)
	}
	for n, in := range tx.Inputs {
		f, err := toFloat(in.ValueIface)
		if err != nil {
			return nil, err
		}
		tx.Inputs[n].Value = f
	}
	for n, out := range tx.Outputs {
		f, err := toFloat(out.ValueIface)
		if err != nil {
			return nil, err
		}
		tx.Outputs[n].Value = f
	}
	return tx, nil
}

func (i *InsightClient) GetRawTransaction(txid string) ([]byte, error) {
	resp, err := i.doRequest("rawtx/"+txid, http.MethodGet, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	tx := new(RawTxResponse)
	if err = json.NewDecoder(resp.Body).Decode(tx); err != nil {
		return nil, fmt.Errorf("error decoding transactions: %s", err)
	}
	return hex.DecodeString(tx.RawTx)
}

func (i *InsightClient) GetTransactions(addrs []btcutil.Address) ([]Transaction, error) {
	var txs []Transaction
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

func (i *InsightClient) getTransactions(addrs []btcutil.Address, from, to int) (*TransactionList, error) {
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
	resp, err := i.doRequest("addrs/txs", http.MethodPost, b, nil)
	if err != nil {
		return nil, err
	}
	tl := new(TransactionList)
	decoder := json.NewDecoder(resp.Body)
	defer resp.Body.Close()
	if err = decoder.Decode(tl); err != nil {
		return nil, fmt.Errorf("error decoding transaction list: %s", err)
	}
	for z, tx := range tl.Items {
		for n, in := range tx.Inputs {
			f, err := toFloat(in.ValueIface)
			if err != nil {
				return nil, err
			}
			tl.Items[z].Inputs[n].Value = f
		}
		for n, out := range tx.Outputs {
			f, err := toFloat(out.ValueIface)
			if err != nil {
				return nil, err
			}
			tl.Items[z].Outputs[n].Value = f
		}
	}
	return tl, nil
}

func (i *InsightClient) GetUtxos(addrs []btcutil.Address) ([]Utxo, error) {
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
	resp, err := i.doRequest("addrs/utxo", http.MethodPost, b, nil)
	if err != nil {
		return nil, err
	}
	utxos := []Utxo{}
	decoder := json.NewDecoder(resp.Body)
	defer resp.Body.Close()
	if err = decoder.Decode(&utxos); err != nil {
		return nil, fmt.Errorf("error decoding utxo list: %s", err)
	}
	for z, u := range utxos {
		f, err := toFloat(u.AmountIface)
		if err != nil {
			return nil, err
		}
		utxos[z].Amount = f
	}
	return utxos, nil
}

func (i *InsightClient) BlockNotify() <-chan Block {
	return i.blockNotifyChan
}

func (i *InsightClient) TransactionNotify() <-chan Transaction {
	return i.txNotifyChan
}

func (i *InsightClient) ListenAddress(addr btcutil.Address) {
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

func (i *InsightClient) setupListeners(u url.URL, proxyDialer proxy.Dialer) {
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
	Log.Infof("Connected to websocket endpoint %s", u.Host)

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
	resp, err := i.doRequest("tx/send", http.MethodPost, txJson, nil)
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

func (i *InsightClient) GetBestBlock() (*Block, error) {
	q, err := url.ParseQuery("limit=2")
	if err != nil {
		return nil, err
	}
	resp, err := i.doRequest("blocks", http.MethodGet, nil, q)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(resp.Body)
	sl := new(BlockList)
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

func (i *InsightClient) GetBlocksBefore(to time.Time, limit int) (*BlockList, error) {
	resp, err := i.doRequest("blocks", http.MethodGet, nil, url.Values{
		"blockDate":      {to.Format("2006-01-02")},
		"startTimestamp": {fmt.Sprint(to.Unix())},
		"limit":          {fmt.Sprint(limit)},
	})
	if err != nil {
		return nil, err
	}
	list := new(BlockList)
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

func (i *InsightClient) EstimateFee(nbBlocks int) (int, error) {
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
