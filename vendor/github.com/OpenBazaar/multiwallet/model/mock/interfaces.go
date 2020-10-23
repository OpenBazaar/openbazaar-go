package mock

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	gosocketio "github.com/OpenBazaar/golang-socketio"
	"github.com/OpenBazaar/multiwallet/client"
	"github.com/OpenBazaar/multiwallet/model"
	"github.com/btcsuite/btcutil"
)

type MockAPIClient struct {
	blockChan chan model.Block
	txChan    chan model.Transaction

	listeningAddrs []btcutil.Address
	chainTip       int
	feePerBlock    int
	info           *model.Info
	addrToScript   func(btcutil.Address) ([]byte, error)
}

func NewMockApiClient(addrToScript func(btcutil.Address) ([]byte, error)) model.APIClient {
	return &MockAPIClient{
		blockChan:    make(chan model.Block),
		txChan:       make(chan model.Transaction),
		chainTip:     0,
		addrToScript: addrToScript,
		feePerBlock:  1,
		info:         &MockInfo,
	}
}

func (m *MockAPIClient) Start() error {
	return nil
}

func (m *MockAPIClient) GetInfo() (*model.Info, error) {
	return m.info, nil
}

func (m *MockAPIClient) GetTransaction(txid string) (*model.Transaction, error) {
	for _, tx := range MockTransactions {
		if tx.Txid == txid {
			return &tx, nil
		}
	}
	return nil, errors.New("Not found")
}

func (m *MockAPIClient) GetRawTransaction(txid string) ([]byte, error) {
	if raw, ok := MockRawTransactions[txid]; ok {
		return raw, nil
	}
	return nil, errors.New("Not found")
}

func (m *MockAPIClient) GetTransactions(addrs []btcutil.Address) ([]model.Transaction, error) {
	txs := make([]model.Transaction, len(MockTransactions))
	copy(txs, MockTransactions)
	txs[0].Outputs[1].ScriptPubKey.Addresses = []string{addrs[0].String()}
	txs[1].Inputs[0].Addr = addrs[0].String()
	txs[1].Outputs[1].ScriptPubKey.Addresses = []string{addrs[1].String()}
	txs[2].Outputs[1].ScriptPubKey.Addresses = []string{addrs[2].String()}
	return txs, nil
}

func (m *MockAPIClient) GetUtxos(addrs []btcutil.Address) ([]model.Utxo, error) {
	utxos := make([]model.Utxo, len(MockUtxos))
	copy(utxos, MockUtxos)
	utxos[0].Address = addrs[1].String()
	script, _ := m.addrToScript(addrs[1])
	utxos[0].ScriptPubKey = hex.EncodeToString(script)
	utxos[1].Address = addrs[2].String()
	script, _ = m.addrToScript(addrs[2])
	utxos[1].ScriptPubKey = hex.EncodeToString(script)
	return utxos, nil
}

func (m *MockAPIClient) BlockNotify() <-chan model.Block {
	return m.blockChan
}

func (m *MockAPIClient) TransactionNotify() <-chan model.Transaction {
	return m.txChan
}

func (m *MockAPIClient) ListenAddresses(addrs ...btcutil.Address) {
	m.listeningAddrs = append(m.listeningAddrs, addrs...)
}

func (m *MockAPIClient) Broadcast(tx []byte) (string, error) {
	return "a8c685478265f4c14dada651969c45a65e1aeb8cd6791f2f5bb6a1d9952104d9", nil
}

func (m *MockAPIClient) GetBestBlock() (*model.Block, error) {
	return &MockBlocks[m.chainTip], nil
}

func (m *MockAPIClient) EstimateFee(nBlocks int) (int, error) {
	return m.feePerBlock * nBlocks, nil
}

func (m *MockAPIClient) Close() {}

func MockWebsocketClientOnClientPool(p *client.ClientPool) *MockSocketClient {
	var (
		callbacksMap     = make(map[string]func(*gosocketio.Channel, interface{}))
		mockSocketClient = &MockSocketClient{
			callbacks:          callbacksMap,
			listeningAddresses: []string{},
		}
	)
	for _, c := range p.Clients() {
		c.SocketClient = mockSocketClient
	}
	return mockSocketClient
}

func NewMockWebsocketClient() *MockSocketClient {
	var (
		callbacksMap     = make(map[string]func(*gosocketio.Channel, interface{}))
		mockSocketClient = &MockSocketClient{
			callbacks:          callbacksMap,
			listeningAddresses: []string{},
		}
	)
	return mockSocketClient
}

type MockSocketClient struct {
	callbackMutex      sync.Mutex
	callbacks          map[string]func(*gosocketio.Channel, interface{})
	listeningAddresses []string
}

func (m *MockSocketClient) SendCallback(method string, args ...interface{}) {
	if gosocketChan, ok := args[0].(*gosocketio.Channel); ok {
		m.callbacks[method](gosocketChan, args[1])
	} else {
		m.callbacks[method](nil, args[1])
	}
}

func (m *MockSocketClient) IsListeningForAddress(addr string) bool {
	for _, a := range m.listeningAddresses {
		if a == addr {
			return true
		}
	}
	return false
}

func (m *MockSocketClient) On(method string, callback interface{}) error {
	c, ok := callback.(func(h *gosocketio.Channel, args interface{}))
	if !ok {
		return fmt.Errorf("failed casting mock callback: %+v", callback)
	}

	m.callbackMutex.Lock()
	defer m.callbackMutex.Unlock()
	if method == "bitcoind/addresstxid" {
		m.callbacks[method] = c
	} else if method == "bitcoind/hashblock" {
		m.callbacks[method] = c
	}
	return nil
}

func (m *MockSocketClient) Emit(method string, args []interface{}) error {
	if method == "subscribe" {
		subscribeTo, ok := args[0].(string)
		if !ok || subscribeTo != "bitcoind/addresstxid" {
			return fmt.Errorf("first emit arg is not bitcoind/addresstxid, was: %+v", args[0])
		}
		addrs, ok := args[1].([]string)
		if !ok {
			return fmt.Errorf("second emit arg is not address value, was %+v", args[1])
		}
		m.listeningAddresses = append(m.listeningAddresses, addrs...)
	}
	return nil
}

func (m *MockSocketClient) Close() {}
