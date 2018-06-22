package client

import (
	"encoding/hex"
	"errors"
	"github.com/OpenBazaar/golang-socketio"
	"github.com/btcsuite/btcutil"
)

var MockInfo = Info{
	Version:         1,
	ProtocolVersion: 9005,
	Blocks:          1289596,
	TimeOffset:      0,
	Connections:     1024,
	DifficultyIface: "1.23",
	Difficulty:      1.23,
	Testnet:         true,
	RelayFeeIface:   "1.00",
	RelayFee:        1.00,
	Errors:          "",
	Network:         "testnet",
}

var MockBlocks = []Block{
	{
		Hash:              "000000000000004c68a477283a8db18c1d1c2155b03d9bc23d587ac5e1c4d1af",
		Height:            1289594,
		PreviousBlockhash: "00000000000003df72ec254d787b216ae913cb82c6ab601c4b3f19fd5d1cf9aa",
		Tx:                make([]string, 21),
		Size:              4705,
		Time:              1522349145,
	},
	{
		Hash:              "0000000000000142ffae87224cb67206e93bf934f9fdeba75d02a7050acc6136",
		Height:            1289595,
		PreviousBlockhash: "000000000000004c68a477283a8db18c1d1c2155b03d9bc23d587ac5e1c4d1af",
		Tx:                make([]string, 30),
		Size:              6623,
		Time:              1522349136,
	},
	{
		Hash:              "000000000000033ef24180d5d282d0e6d03b1185e29421fda97e1ba0ffd7c918",
		Height:            1289596,
		PreviousBlockhash: "0000000000000142ffae87224cb67206e93bf934f9fdeba75d02a7050acc6136",
		Tx:                make([]string, 5),
		Size:              1186,
		Time:              1522349156,
	},
}

var MockTransactions = []Transaction{
	{
		Txid:     "54ebaa07c42216393b9d5816e40dd608593b92c42e2d6525f45bdd36bce8fe4d",
		Version:  2,
		Locktime: 512378,
		Inputs: []Input{
			{
				Txid:       "6d892f04fc097f430d58ab06229c9b6344a130fc1842da5b990e857daed42194",
				Vout:       1,
				Sequence:   1,
				ValueIface: "0.04294455",
				Value:      0.04294455,
				N:          0,
				ScriptSig: Script{
					Hex: "4830450221008665481674067564ef562cfd8d1ca8f1506133fb26a2319e4b8dfba3cedfd5de022038f27121c44e6c64b93b94d72620e11b9de35fd864730175db9176ca98f1ec610121022023e49335a0dddb864ff673468a6cc04e282571b1227933fcf3ff9babbcc662",
				},
				Addr:     "1C74Gbij8Q5h61W58aSKGvXK4rk82T2A3y",
				Satoshis: 4294455,
			},
		},
		Outputs: []Output{
			{
				ScriptPubKey: OutScript{
					Script: Script{
						Hex: "76a914ff3f7d402fbd6d116ba4a02af9784f3ae9b7108a88ac",
					},
					Type:      "pay-to-pubkey-hash",
					Addresses: []string{"1QGdNEDjWnghrjfTBCTDAPZZ3ffoKvGc9B"},
				},
				ValueIface: "0.01398175",
				Value:      0.01398175,
				N:          0,
			},
			{
				ScriptPubKey: OutScript{
					Script: Script{
						Hex: "76a914f99b84270843bdab59a71ce9af15b89bef5087a388ac",
					},
					Type:      "pay-to-pubkey-hash",
					Addresses: []string{"1PkoZDtXT63BnYGd429Vy4DoyGhdDcjQiN"}, // var
				},
				ValueIface: "0.02717080",
				Value:      0.02717080,
				N:          1,
			},
		},
		Time:          1520449061,
		BlockHash:     "0000000000000000003f1fb88ac3dab0e607e87def0e9031f7bea02cb464a04f",
		BlockHeight:   1289475,
		Confirmations: 15,
	},
	{
		Txid:     "ff2b865c3b73439912eebf4cce9a15b12c7d7bcdd14ae1110a90541426c4e7c5",
		Version:  2,
		Locktime: 0,
		Inputs: []Input{
			{
				Txid:       "54ebaa07c42216393b9d5816e40dd608593b92c42e2d6525f45bdd36bce8fe4d",
				Vout:       1,
				Sequence:   1,
				ValueIface: "0.02717080",
				Value:      0.02717080,
				N:          0,
				ScriptSig: Script{
					Hex: "4830450221008665481674067564ef562cfd8d1ca8f1506133fb26a2319e4b8dfba3cedfd5de022038f27121c44e6c64b93b94d72620e11b9de35fd864730175db9176ca98f1ec610121022023e49335a0dddb864ff673468a6cc04e282571b1227933fcf3ff9babbcc662",
				},
				Addr:     "1PkoZDtXT63BnYGd429Vy4DoyGhdDcjQiN", // var tx0:1
				Satoshis: 2717080,
			},
		},
		Outputs: []Output{
			{
				ScriptPubKey: OutScript{
					Script: Script{
						Hex: "a9144b18dadba74ad5ef4dbbfea47f9d5aaefe766c6387",
					},
					Type:      "pay-to-script-hash",
					Addresses: []string{"38Y6Nt35hQcEDxyCfCEi62QLGPnr4mhANc"},
				},
				ValueIface: "0.01398175",
				Value:      0.01617080,
				N:          0,
			},
			{
				ScriptPubKey: OutScript{
					Script: Script{
						Hex: "76a914f821d6db9376dc60124de46a8683110877e1f13188ac",
					},
					Type:      "pay-to-pubkey-hash",
					Addresses: []string{"1Pd17mbYsVPcCKLtNdPkngtizTj7zjzqeK"}, // var change
				},
				ValueIface: "0.01",
				Value:      0.01,
				N:          1,
			},
		},
		Time:          1520449061,
		BlockHash:     "0000000000000000003f1fb88ac3dab0e607e87def0e9031f7bea02cb464a04f",
		BlockHeight:   1289475,
		Confirmations: 10,
	},
	{
		Txid:     "7fe0f12c1f77b33128c1b4a79fcc1f723c5be90dd1216b0664a8307060d4345e",
		Version:  2,
		Locktime: 0,
		Inputs: []Input{
			{
				Txid:       "e940d490e7e13985d7382676b5a6699215e4e9e5bb521ec61e81ac3ab12e34ff",
				Vout:       1,
				Sequence:   1,
				ValueIface: "0.3",
				Value:      0.3,
				N:          0,
				ScriptSig: Script{
					Hex: "4830450221008665481674067564ef562cfd8d1ca8f1506133fb26a2319e4b8dfba3cedfd5de022038f27121c44e6c64b93b94d72620e11b9de35fd864730175db9176ca98f1ec610121022023e49335a0dddb864ff673468a6cc04e282571b1227933fcf3ff9babbcc662",
				},
				Addr:     "1H2ZS69jUZz6CuCtiRCTWXr4AhAWfXc4YT",
				Satoshis: 2717080,
			},
		},
		Outputs: []Output{
			{
				ScriptPubKey: OutScript{
					Script: Script{
						Hex: "76a914e20c0ca5875b1fb0d057e23d032ba88b9dda6f3888ac",
					},
					Type:      "pay-to-pubkey-hash",
					Addresses: []string{"1McE9ZXFhWkFeAqR1hyAm1XaDK8zvyrFPr"},
				},
				ValueIface: "0.2",
				Value:      0.2,
				N:          0,
			},
			{
				ScriptPubKey: OutScript{
					Script: Script{
						Hex: "76a914594963287fe6684872340e9078a78d0accbec26288ac",
					},
					Type:      "pay-to-pubkey-hash",
					Addresses: []string{"199747e2arXMBPiWfTqpBTXz3eFbeJPMqS"}, // var
				},
				ValueIface: "0.1",
				Value:      0.1,
				N:          1,
			},
		},
		Time:          1520449061,
		BlockHash:     "0000000000000000003f1fb88ac3dab0e607e87def0e9031f7bea02cb464a04f",
		BlockHeight:   1289475,
		Confirmations: 2,
	},
	{
		Txid:     "830bf683ab8eec1a75d891689e2989f846508bc7d500cb026ef671c2d1dce20c",
		Version:  2,
		Locktime: 516299,
		Inputs: []Input{
			{
				Txid:       "b466d034076ab53f4b019d573b6c68cf68c5b9a8cfbf07c8d46208d0fcf37762",
				Vout:       0,
				Sequence:   4294967294,
				ValueIface: "0.01983741",
				Value:      0.01983741,
				N:          0,
				ScriptSig: Script{
					Hex: "483045022100baa2b3653d48ccf2838caa549d96a40540c838c4f4a8e7048dbe158ec180b3f602206f1bb8c6d055103ce635db562c31ebd8c30565c5d415458affb9f99407ec06d10121039fea462cb64296e01384cffc16af4b86ab14b6027094399bf5a4b52e5c9ffef3",
				},
				Addr:     "1LUv9VNMZQR4VknWj1TBa1oDgPq53wP7BK",
				Satoshis: 1983741,
			},
		},
		Outputs: []Output{
			{
				ScriptPubKey: OutScript{
					Script: Script{
						Hex: "76a91491a8a9e0375f10b721743782162a0b4f9fae69a888ac",
					},
					Type:      "pay-to-pubkey-hash",
					Addresses: []string{"1EHB2mSaUXzkM6r6XgVHcutFDZoB9e2mZH"},
				},
				ValueIface: "0.01181823",
				Value:      0.01181823,
				N:          0,
			},
			{
				ScriptPubKey: OutScript{
					Script: Script{
						Hex: "a91457fc729da2a83dc8cd3c1835351c4a813c2ae8ba87",
					},
					Type:      "pay-to-script-hash",
					Addresses: []string{"39iF8cDMhctrPVoPbi2Vb1NnErg6CEB7BZ"},
				},
				ValueIface: "0.00751918",
				Value:      0.00751918,
				N:          1,
			},
		},
		Time:          1520449061,
		BlockHash:     "0000000000000000003f1fb88ac3dab0e607e87def0e9031f7bea02cb464a04f",
		BlockHeight:   1289475,
		Confirmations: 2,
	},
}

var MockRawTransactions = map[string][]byte{}

var MockUtxos = []Utxo{
	{
		Address:       "1Pd17mbYsVPcCKLtNdPkngtizTj7zjzqeK", // tx1:1
		ScriptPubKey:  "76a914f821d6db9376dc60124de46a8683110877e1f13188ac",
		Vout:          1,
		Satoshis:      1000000,
		Confirmations: 10,
		Txid:          "ff2b865c3b73439912eebf4cce9a15b12c7d7bcdd14ae1110a90541426c4e7c5",
		AmountIface:   "0.01",
		Amount:        0.01,
	},
	{
		Address:       "199747e2arXMBPiWfTqpBTXz3eFbeJPMqS", //tx2:1
		ScriptPubKey:  "76a914594963287fe6684872340e9078a78d0accbec26288ac",
		Vout:          1,
		Satoshis:      10000000,
		Confirmations: 2,
		Txid:          "7fe0f12c1f77b33128c1b4a79fcc1f723c5be90dd1216b0664a8307060d4345e",
		AmountIface:   "0.1",
		Amount:        0.1,
	},
	{
		Address:       "39iF8cDMhctrPVoPbi2Vb1NnErg6CEB7BZ",
		ScriptPubKey:  "a91457fc729da2a83dc8cd3c1835351c4a813c2ae8ba87",
		Vout:          1,
		Satoshis:      751918,
		Confirmations: 2,
		Txid:          "830bf683ab8eec1a75d891689e2989f846508bc7d500cb026ef671c2d1dce20c",
		AmountIface:   "0.00751918",
		Amount:        0.00751918,
	},
}

type MockAPIClient struct {
	blockChan chan Block
	txChan    chan Transaction

	listeningAddrs []btcutil.Address
	chainTip       int
	feePerBlock    int
	info           *Info
	addrToScript   func(btcutil.Address) ([]byte, error)
}

func NewMockApiClient(addrToScript func(btcutil.Address) ([]byte, error)) APIClient {
	return &MockAPIClient{
		blockChan:    make(chan Block),
		txChan:       make(chan Transaction),
		chainTip:     0,
		addrToScript: addrToScript,
		feePerBlock:  1,
		info:         &MockInfo,
	}
}

func (m *MockAPIClient) GetInfo() (*Info, error) {
	return m.info, nil
}

func (m *MockAPIClient) GetTransaction(txid string) (*Transaction, error) {
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

func (m *MockAPIClient) GetTransactions(addrs []btcutil.Address) ([]Transaction, error) {
	txs := make([]Transaction, len(MockTransactions))
	copy(txs, MockTransactions)
	txs[0].Outputs[1].ScriptPubKey.Addresses = []string{addrs[0].String()}
	txs[1].Inputs[0].Addr = addrs[0].String()
	txs[1].Outputs[1].ScriptPubKey.Addresses = []string{addrs[1].String()}
	txs[2].Outputs[1].ScriptPubKey.Addresses = []string{addrs[2].String()}
	return txs, nil
}

func (m *MockAPIClient) GetUtxos(addrs []btcutil.Address) ([]Utxo, error) {
	utxos := make([]Utxo, len(MockUtxos))
	copy(utxos, MockUtxos)
	utxos[0].Address = addrs[1].String()
	script, _ := m.addrToScript(addrs[1])
	utxos[0].ScriptPubKey = hex.EncodeToString(script)
	utxos[1].Address = addrs[2].String()
	script, _ = m.addrToScript(addrs[2])
	utxos[1].ScriptPubKey = hex.EncodeToString(script)
	return utxos, nil
}

func (m *MockAPIClient) BlockNotify() <-chan Block {
	return m.blockChan
}

func (m *MockAPIClient) TransactionNotify() <-chan Transaction {
	return m.txChan
}

func (m *MockAPIClient) ListenAddress(addr btcutil.Address) {
	m.listeningAddrs = append(m.listeningAddrs, addr)
}

func (m *MockAPIClient) Broadcast(tx []byte) (string, error) {
	return "txid", nil
}

func (m *MockAPIClient) GetBestBlock() (*Block, error) {
	return &MockBlocks[m.chainTip], nil
}

func (m *MockAPIClient) EstimateFee(nBlocks int) (int, error) {
	return m.feePerBlock * nBlocks, nil
}

func (m *MockAPIClient) Close() {}

type MockSocketClient struct {
	callbacks          map[string]func(h *gosocketio.Channel, args interface{})
	listeningAddresses []string
}

func (m *MockSocketClient) On(method string, callback interface{}) error {
	c, ok := callback.(func(h *gosocketio.Channel, args interface{}))
	if !ok {
		return nil
	}

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
			return nil
		}
		addrs, ok := args[1].([]string)
		if !ok {
			return nil
		}
		for _, addr := range addrs {
			m.listeningAddresses = append(m.listeningAddresses, addr)
		}
	}
	return nil
}

func (m *MockSocketClient) Close() {}
