package zcashd

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	btcrpcclient "github.com/btcsuite/btcd/rpcclient"
)

type NotificationListener struct {
	client    *btcrpcclient.Client
	listeners []func(wallet.TransactionCallback)
	params    *chaincfg.Params
}

func (l *NotificationListener) notify(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	txid := string(b)
	hash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		log.Error(err)
		return
	}
	tx, err := l.client.GetRawTransaction(hash)
	if err != nil {
		log.Error(err)
		return
	}
	watchOnly := false
	txInfo, err := l.client.GetTransaction(hash, &watchOnly)
	if err != nil {
		watchOnly = true
	}
	var outputs []wallet.TransactionOutput
	for i, txout := range tx.MsgTx().TxOut {
		addr, _ := scriptToAddress(txout.PkScript, l.params)
		out := wallet.TransactionOutput{Address: addr, Value: txout.Value, Index: uint32(i)}
		outputs = append(outputs, out)
	}
	var inputs []wallet.TransactionInput
	for _, txin := range tx.MsgTx().TxIn {
		in := wallet.TransactionInput{OutpointHash: txin.PreviousOutPoint.Hash.CloneBytes(), OutpointIndex: txin.PreviousOutPoint.Index}
		prev, err := l.client.GetRawTransaction(&txin.PreviousOutPoint.Hash)
		if err != nil {
			inputs = append(inputs, in)
			continue
		}
		addr, _ := scriptToAddress(prev.MsgTx().TxOut[txin.PreviousOutPoint.Index].PkScript, l.params)
		in.LinkedAddress = addr
		in.Value = prev.MsgTx().TxOut[txin.PreviousOutPoint.Index].Value
		inputs = append(inputs, in)
	}

	height := int32(0)
	if txInfo.Confirmations > 0 {
		hash, err := chainhash.NewHashFromStr(txInfo.BlockHash)
		if err != nil {
			log.Error(err)
			return
		}
		h := ``
		if hash != nil {
			h += `"` + hash.String() + `"`
		}
		resp, err := l.client.RawRequest("getblockheader", []json.RawMessage{json.RawMessage(h)})
		if err != nil {
			log.Error(err)
			return
		}
		type Respose struct {
			Height int32 `json:"height"`
		}
		r := new(Respose)
		err = json.Unmarshal([]byte(resp), r)
		if err != nil {
			log.Error(err)
			return
		}
		height = r.Height
	}
	cb := wallet.TransactionCallback{
		Txid:      tx.Hash().String(),
		Inputs:    inputs,
		Outputs:   outputs,
		WatchOnly: watchOnly,
		Value:     int64(txInfo.Amount * 100000000),
		Timestamp: time.Unix(txInfo.TimeReceived, 0),
		Height:    height,
	}
	for _, lis := range l.listeners {
		lis(cb)
	}
}

func StartNotificationListener(client *btcrpcclient.Client, params *chaincfg.Params, listeners []func(wallet.TransactionCallback)) {
	l := NotificationListener{
		client:    client,
		listeners: listeners,
		params:    params,
	}
	http.HandleFunc("/", l.notify)
	http.ListenAndServe(":8330", nil)
}
