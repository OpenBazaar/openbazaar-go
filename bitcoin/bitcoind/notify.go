package bitcoind

import (
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcrpcclient"
	"io/ioutil"
	"net/http"
	"time"
)

type NotificationListener struct {
	client    *btcrpcclient.Client
	listeners []func(spvwallet.TransactionCallback)
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
	var outputs []spvwallet.TransactionOutput
	for i, txout := range tx.MsgTx().TxOut {
		out := spvwallet.TransactionOutput{ScriptPubKey: txout.PkScript, Value: txout.Value, Index: uint32(i)}
		outputs = append(outputs, out)
	}
	var inputs []spvwallet.TransactionInput
	for _, txin := range tx.MsgTx().TxIn {
		in := spvwallet.TransactionInput{OutpointHash: txin.PreviousOutPoint.Hash.CloneBytes(), OutpointIndex: txin.PreviousOutPoint.Index}
		prev, err := l.client.GetRawTransaction(&txin.PreviousOutPoint.Hash)
		if err != nil {
			inputs = append(inputs, in)
			continue
		}
		in.LinkedScriptPubKey = prev.MsgTx().TxOut[txin.PreviousOutPoint.Index].PkScript
		in.Value = prev.MsgTx().TxOut[txin.PreviousOutPoint.Index].Value
		inputs = append(inputs, in)
	}
	cb := spvwallet.TransactionCallback{
		Txid:      tx.Hash().CloneBytes(),
		Inputs:    inputs,
		Outputs:   outputs,
		WatchOnly: watchOnly,
		Value:     int64(txInfo.Amount * 100000000),
		Timestamp: time.Unix(txInfo.TimeReceived, 0),
		Height:    int32(txInfo.BlockIndex),
	}
	for _, lis := range l.listeners {
		lis(cb)
	}
}

func startNotificationListener(client *btcrpcclient.Client, listeners []func(spvwallet.TransactionCallback)) {
	l := NotificationListener{
		client:    client,
		listeners: listeners,
	}
	http.HandleFunc("/", l.notify)
	http.ListenAndServe(":8330", nil)
}
