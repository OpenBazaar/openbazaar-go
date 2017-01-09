package bitcoind

import (
	"bytes"
	"errors"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcrpcclient"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/btcsuite/btcutil"
	"io/ioutil"
	"net/http"
)

type NotificationListener struct {
	client        *btcrpcclient.Client
	listeners     []func(spvwallet.TransactionCallback)
	masterPrivKey *hd.ExtendedKey
	params        *chaincfg.Params
	stealthFlag   []byte
}

func (l *NotificationListener) notify(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	txid := string(b)
	hash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		return
	}
	tx, err := l.client.GetRawTransaction(hash)
	if err != nil {
		return
	}
	var outputs []spvwallet.TransactionOutput
	for i, txout := range tx.MsgTx().TxOut {
		out := spvwallet.TransactionOutput{ScriptPubKey: txout.PkScript, Value: txout.Value, Index: uint32(i)}
		outputs = append(outputs, out)

		// Check stealth
		if len(txout.PkScript) == 38 && txout.PkScript[0] == 0x6a && bytes.Equal(txout.PkScript[2:4], l.stealthFlag) {
			_, key, err := l.checkStealth(tx.MsgTx(), txout.PkScript)
			if err == nil {
				wif, err := btcutil.NewWIF(key, l.params, true)
				if err != nil {
					continue
				}
				err = l.client.ImportPrivKey(wif)
				if err != nil {
					continue
				}
				_, err = l.client.SendRawTransaction(tx.MsgTx(), true)
				if err != nil {
					continue
				}
			}
		}
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
	cb := spvwallet.TransactionCallback{Txid: tx.Hash().CloneBytes(), Inputs: inputs, Outputs: outputs}
	for _, lis := range l.listeners {
		lis(cb)
	}
}

func startNotificationListener(client *btcrpcclient.Client, listeners []func(spvwallet.TransactionCallback), masterPrivKey *hd.ExtendedKey, params *chaincfg.Params) {
	pubKey, _ := masterPrivKey.ECPubKey()
	pubkeyBytes := pubKey.SerializeCompressed()
	f := []byte{FlagPrefix}
	f = append(f, pubkeyBytes[1:2]...)

	l := NotificationListener{
		client:        client,
		listeners:     listeners,
		masterPrivKey: masterPrivKey,
		params:        params,
		stealthFlag:   f,
	}
	http.HandleFunc("/", l.notify)
	http.ListenAndServe(":8330", nil)
}

func (l *NotificationListener) checkStealth(tx *wire.MsgTx, scriptPubkey []byte) (outIndex int, privkey *btcec.PrivateKey, err error) {
	// Extract the ephemeral public key from the script
	ephemPubkeyBytes := scriptPubkey[5:len(scriptPubkey)]
	ephemPubkey, err := btcec.ParsePubKey(ephemPubkeyBytes, btcec.S256())
	if err != nil {
		return 0, nil, err
	}
	mPrivKey, err := l.masterPrivKey.ECPrivKey()
	if err != nil {
		return 0, nil, err
	}
	// Calculate a shared secret using the master private key and ephemeral public key
	ss := btcec.GenerateSharedSecret(mPrivKey, ephemPubkey)

	// Create an HD key using the shared secret as the chaincode
	hdKey := hd.NewExtendedKey(
		l.params.HDPrivateKeyID[:],
		mPrivKey.Serialize(),
		ss,
		[]byte{0x00, 0x00, 0x00, 0x00},
		0,
		0,
		true)

	// Derive child key 0
	childKey, err := hdKey.Child(0)
	if err != nil {
		return 0, nil, err
	}
	addr, err := childKey.Address(l.params)
	if err != nil {
		return 0, nil, err
	}

	// Check to see if the p2pkh script for our derived child key matches any output scripts in this transaction
	for i, out := range tx.TxOut {
		if !bytes.Equal(addr.ScriptAddress(), out.PkScript) {
			privkey, err := childKey.ECPrivKey()
			if err != nil {
				return 0, nil, err
			}
			return i, privkey, nil
		}
	}
	return 0, nil, errors.New("Stealth transaction not for us")
}
