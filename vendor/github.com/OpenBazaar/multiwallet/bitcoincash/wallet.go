package bitcoincash

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/op/go-logging"
	"io"
	"log"
	"math/big"
	"strconv"
	"time"

	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/btcsuite/btcwallet/wallet/txrules"
	"github.com/cpacia/bchutil"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/net/proxy"

	"github.com/OpenBazaar/multiwallet/cache"
	"github.com/OpenBazaar/multiwallet/client"
	"github.com/OpenBazaar/multiwallet/config"
	"github.com/OpenBazaar/multiwallet/keys"
	"github.com/OpenBazaar/multiwallet/model"
	"github.com/OpenBazaar/multiwallet/service"
	"github.com/OpenBazaar/multiwallet/util"
)

type BitcoinCashWallet struct {
	db     wi.Datastore
	km     *keys.KeyManager
	params *chaincfg.Params
	client model.APIClient
	ws     *service.WalletService
	fp     *util.FeeProvider

	mPrivKey *hd.ExtendedKey
	mPubKey  *hd.ExtendedKey

	exchangeRates wi.ExchangeRates
	log           *logging.Logger
}

var (
	_                             = wi.Wallet(&BitcoinCashWallet{})
	BitcoinCashCurrencyDefinition = wi.CurrencyDefinition{
		Code:         "BCH",
		Divisibility: 8,
	}
)

func NewBitcoinCashWallet(cfg config.CoinConfig, mnemonic string, params *chaincfg.Params, proxy proxy.Dialer, cache cache.Cacher, disableExchangeRates bool) (*BitcoinCashWallet, error) {
	seed := bip39.NewSeed(mnemonic, "")

	mPrivKey, err := hd.NewMaster(seed, params)
	if err != nil {
		return nil, err
	}
	mPubKey, err := mPrivKey.Neuter()
	if err != nil {
		return nil, err
	}
	km, err := keys.NewKeyManager(cfg.DB.Keys(), params, mPrivKey, wi.BitcoinCash, bitcoinCashAddress)
	if err != nil {
		return nil, err
	}

	c, err := client.NewClientPool(cfg.ClientAPIs, proxy)
	if err != nil {
		return nil, err
	}

	wm, err := service.NewWalletService(cfg.DB, km, c, params, wi.BitcoinCash, cache)
	if err != nil {
		return nil, err
	}
	exchangeRates := NewBitcoinCashPriceFetcher(proxy)
	if !disableExchangeRates {
		go exchangeRates.Run()
	}

	fp := util.NewFeeProvider(cfg.MaxFee, cfg.HighFee, cfg.MediumFee, cfg.LowFee, cfg.SuperLowFee, exchangeRates)

	return &BitcoinCashWallet{
		db:            cfg.DB,
		km:            km,
		params:        params,
		client:        c,
		ws:            wm,
		fp:            fp,
		mPrivKey:      mPrivKey,
		mPubKey:       mPubKey,
		exchangeRates: exchangeRates,
		log:           logging.MustGetLogger("bitcoin-cash-wallet"),
	}, nil
}

func bitcoinCashAddress(key *hd.ExtendedKey, params *chaincfg.Params) (btcutil.Address, error) {
	addr, err := key.Address(params)
	if err != nil {
		return nil, err
	}
	return bchutil.NewCashAddressPubKeyHash(addr.ScriptAddress(), params)
}

func (w *BitcoinCashWallet) Start() {
	w.client.Start()
	w.ws.Start()
}

func (w *BitcoinCashWallet) Params() *chaincfg.Params {
	return w.params
}

func (w *BitcoinCashWallet) CurrencyCode() string {
	if w.params.Name == chaincfg.MainNetParams.Name {
		return "bch"
	} else {
		return "tbch"
	}
}

func (w *BitcoinCashWallet) IsDust(amount big.Int) bool {
	if !amount.IsInt64() || amount.Cmp(big.NewInt(0)) <= 0 {
		return false
	}
	return txrules.IsDustAmount(btcutil.Amount(amount.Int64()), 25, txrules.DefaultRelayFeePerKb)
}

func (w *BitcoinCashWallet) MasterPrivateKey() *hd.ExtendedKey {
	return w.mPrivKey
}

func (w *BitcoinCashWallet) MasterPublicKey() *hd.ExtendedKey {
	return w.mPubKey
}

func (w *BitcoinCashWallet) ChildKey(keyBytes []byte, chaincode []byte, isPrivateKey bool) (*hd.ExtendedKey, error) {
	parentFP := []byte{0x00, 0x00, 0x00, 0x00}
	var id []byte
	if isPrivateKey {
		id = w.params.HDPrivateKeyID[:]
	} else {
		id = w.params.HDPublicKeyID[:]
	}
	hdKey := hd.NewExtendedKey(
		id,
		keyBytes,
		chaincode,
		parentFP,
		0,
		0,
		isPrivateKey)
	return hdKey.Child(0)
}

func (w *BitcoinCashWallet) CurrentAddress(purpose wi.KeyPurpose) btcutil.Address {
	key, err := w.km.GetCurrentKey(purpose)
	if err != nil {
		w.log.Errorf("Error generating current key: %s", err)
	}
	addr, err := w.km.KeyToAddress(key)
	if err != nil {
		w.log.Errorf("Error converting key to address: %s", err)
	}
	return addr
}

func (w *BitcoinCashWallet) NewAddress(purpose wi.KeyPurpose) btcutil.Address {
	key, err := w.km.GetNextUnused(purpose)
	if err != nil {
		w.log.Errorf("Error generating next unused key: %s", err)
	}
	addr, err := w.km.KeyToAddress(key)
	if err != nil {
		w.log.Errorf("Error converting key to address: %s", err)
	}
	if err := w.db.Keys().MarkKeyAsUsed(addr.ScriptAddress()); err != nil {
		w.log.Errorf("Error marking key as used: %s", err)
	}
	return addr
}

func (w *BitcoinCashWallet) DecodeAddress(addr string) (btcutil.Address, error) {
	return bchutil.DecodeAddress(addr, w.params)
}

func (w *BitcoinCashWallet) ScriptToAddress(script []byte) (btcutil.Address, error) {
	return bchutil.ExtractPkScriptAddrs(script, w.params)
}

func (w *BitcoinCashWallet) AddressToScript(addr btcutil.Address) ([]byte, error) {
	return bchutil.PayToAddrScript(addr)
}

func (w *BitcoinCashWallet) HasKey(addr btcutil.Address) bool {
	_, err := w.km.GetKeyForScript(addr.ScriptAddress())
	if err != nil {
		return false
	}
	return true
}

func (w *BitcoinCashWallet) Balance() (wi.CurrencyValue, wi.CurrencyValue) {
	utxos, _ := w.db.Utxos().GetAll()
	txns, _ := w.db.Txns().GetAll(false)
	c, u := util.CalcBalance(utxos, txns)
	return wi.CurrencyValue{Value: *big.NewInt(c), Currency: BitcoinCashCurrencyDefinition},
		wi.CurrencyValue{Value: *big.NewInt(u), Currency: BitcoinCashCurrencyDefinition}
}

func (w *BitcoinCashWallet) Transactions() ([]wi.Txn, error) {
	height, _ := w.ChainTip()
	txns, err := w.db.Txns().GetAll(false)
	if err != nil {
		return txns, err
	}
	for i, tx := range txns {
		var confirmations int32
		var status wi.StatusCode
		confs := int32(height) - tx.Height + 1
		if tx.Height <= 0 {
			confs = tx.Height
		}
		switch {
		case confs < 0:
			status = wi.StatusDead
		case confs == 0 && time.Since(tx.Timestamp) <= time.Hour*6:
			status = wi.StatusUnconfirmed
		case confs == 0 && time.Since(tx.Timestamp) > time.Hour*6:
			status = wi.StatusDead
		case confs > 0 && confs < 6:
			status = wi.StatusPending
			confirmations = confs
		case confs > 5:
			status = wi.StatusConfirmed
			confirmations = confs
		}
		tx.Confirmations = int64(confirmations)
		tx.Status = status
		txns[i] = tx
	}
	return txns, nil
}

func (w *BitcoinCashWallet) GetTransaction(txid string) (wi.Txn, error) {
	txn, err := w.db.Txns().Get(txid)
	if err == nil {
		tx := wire.NewMsgTx(1)
		rbuf := bytes.NewReader(txn.Bytes)
		err := tx.BtcDecode(rbuf, wire.ProtocolVersion, wire.WitnessEncoding)
		if err != nil {
			return txn, err
		}
		outs := []wi.TransactionOutput{}
		for i, out := range tx.TxOut {
			addr, err := bchutil.ExtractPkScriptAddrs(out.PkScript, w.params)
			if err != nil {
				log.Printf("error extracting address from txn pkscript: %v\n", err)
			}
			tout := wi.TransactionOutput{
				Address: addr,
				Value:   *big.NewInt(out.Value),
				Index:   uint32(i),
			}
			outs = append(outs, tout)
		}
		txn.Outputs = outs
	}
	return txn, err
}

func (w *BitcoinCashWallet) ChainTip() (uint32, string) {
	return w.ws.ChainTip()
}

func (w *BitcoinCashWallet) GetFeePerByte(feeLevel wi.FeeLevel) big.Int {
	return *big.NewInt(int64(w.fp.GetFeePerByte(feeLevel)))
}

func (w *BitcoinCashWallet) Spend(amount big.Int, addr btcutil.Address, feeLevel wi.FeeLevel, referenceID string, spendAll bool) (string, error) {
	var (
		tx  *wire.MsgTx
		err error
	)
	if spendAll {
		tx, err = w.buildSpendAllTx(addr, feeLevel)
		if err != nil {
			return "", err
		}
	} else {
		tx, err = w.buildTx(amount.Int64(), addr, feeLevel, nil)
		if err != nil {
			return "", err
		}
	}

	// Broadcast
	if err := w.Broadcast(tx); err != nil {
		return "", err
	}

	ch := tx.TxHash()
	return ch.String(), nil
}

func (w *BitcoinCashWallet) BumpFee(txid string) (string, error) {
	return w.bumpFee(txid)
}

func (w *BitcoinCashWallet) EstimateFee(ins []wi.TransactionInput, outs []wi.TransactionOutput, feePerByte big.Int) big.Int {
	tx := new(wire.MsgTx)
	for _, out := range outs {
		scriptPubKey, _ := bchutil.PayToAddrScript(out.Address)
		output := wire.NewTxOut(out.Value.Int64(), scriptPubKey)
		tx.TxOut = append(tx.TxOut, output)
	}
	estimatedSize := EstimateSerializeSize(len(ins), tx.TxOut, false, P2PKH)
	fee := estimatedSize * int(feePerByte.Int64())
	return *big.NewInt(int64(fee))
}

func (w *BitcoinCashWallet) EstimateSpendFee(amount big.Int, feeLevel wi.FeeLevel) (big.Int, error) {
	val, err := w.estimateSpendFee(amount.Int64(), feeLevel)
	return *big.NewInt(int64(val)), err
}

func (w *BitcoinCashWallet) SweepAddress(ins []wi.TransactionInput, address *btcutil.Address, key *hd.ExtendedKey, redeemScript *[]byte, feeLevel wi.FeeLevel) (string, error) {
	return w.sweepAddress(ins, address, key, redeemScript, feeLevel)
}

func (w *BitcoinCashWallet) CreateMultisigSignature(ins []wi.TransactionInput, outs []wi.TransactionOutput, key *hd.ExtendedKey, redeemScript []byte, feePerByte big.Int) ([]wi.Signature, error) {
	return w.createMultisigSignature(ins, outs, key, redeemScript, feePerByte.Uint64())
}

func (w *BitcoinCashWallet) Multisign(ins []wi.TransactionInput, outs []wi.TransactionOutput, sigs1 []wi.Signature, sigs2 []wi.Signature, redeemScript []byte, feePerByte big.Int, broadcast bool) ([]byte, error) {
	return w.multisign(ins, outs, sigs1, sigs2, redeemScript, feePerByte.Uint64(), broadcast)
}

func (w *BitcoinCashWallet) GenerateMultisigScript(keys []hd.ExtendedKey, threshold int, timeout time.Duration, timeoutKey *hd.ExtendedKey) (addr btcutil.Address, redeemScript []byte, err error) {
	return w.generateMultisigScript(keys, threshold, timeout, timeoutKey)
}

func (w *BitcoinCashWallet) AddWatchedAddresses(addrs ...btcutil.Address) error {

	var watchedScripts [][]byte
	for _, addr := range addrs {
		if !w.HasKey(addr) {
			script, err := w.AddressToScript(addr)
			if err != nil {
				return err
			}
			watchedScripts = append(watchedScripts, script)
		}
	}

	err := w.db.WatchedScripts().PutAll(watchedScripts)
	if err != nil {
		return err
	}

	w.client.ListenAddresses(addrs...)
	return nil
}

func (w *BitcoinCashWallet) AddWatchedScript(script []byte) error {
	err := w.db.WatchedScripts().Put(script)
	if err != nil {
		return err
	}
	addr, err := w.ScriptToAddress(script)
	if err != nil {
		return err
	}
	w.client.ListenAddresses(addr)
	return nil
}

func (w *BitcoinCashWallet) AddTransactionListener(callback func(wi.TransactionCallback)) {
	w.ws.AddTransactionListener(callback)
}

func (w *BitcoinCashWallet) ReSyncBlockchain(fromTime time.Time) {
	go w.ws.UpdateState()
}

func (w *BitcoinCashWallet) GetConfirmations(txid string) (uint32, uint32, error) {
	txn, err := w.db.Txns().Get(txid)
	if err != nil {
		return 0, 0, err
	}
	if txn.Height == 0 {
		return 0, 0, nil
	}
	chainTip, _ := w.ChainTip()
	return chainTip - uint32(txn.Height) + 1, uint32(txn.Height), nil
}

func (w *BitcoinCashWallet) Close() {
	w.ws.Stop()
	w.client.Close()
}

func (w *BitcoinCashWallet) ExchangeRates() wi.ExchangeRates {
	return w.exchangeRates
}

func (w *BitcoinCashWallet) DumpTables(wr io.Writer) {
	fmt.Fprintln(wr, "Transactions-----")
	txns, _ := w.db.Txns().GetAll(true)
	for _, tx := range txns {
		fmt.Fprintf(wr, "Hash: %s, Height: %d, Value: %s, WatchOnly: %t\n", tx.Txid, int(tx.Height), tx.Value, tx.WatchOnly)
	}
	fmt.Fprintln(wr, "\nUtxos-----")
	utxos, _ := w.db.Utxos().GetAll()
	for _, u := range utxos {
		fmt.Fprintf(wr, "Hash: %s, Index: %d, Height: %d, Value: %s, WatchOnly: %t\n", u.Op.Hash.String(), int(u.Op.Index), int(u.AtHeight), u.Value, u.WatchOnly)
	}
}

// Build a client.Transaction so we can ingest it into the wallet service then broadcast
func (w *BitcoinCashWallet) Broadcast(tx *wire.MsgTx) error {
	var buf bytes.Buffer
	tx.BtcEncode(&buf, wire.ProtocolVersion, wire.BaseEncoding)
	cTxn := model.Transaction{
		Txid:          tx.TxHash().String(),
		Locktime:      int(tx.LockTime),
		Version:       int(tx.Version),
		Confirmations: 0,
		Time:          time.Now().Unix(),
		RawBytes:      buf.Bytes(),
	}
	utxos, err := w.db.Utxos().GetAll()
	if err != nil {
		return err
	}
	for n, in := range tx.TxIn {
		var u wi.Utxo
		for _, ut := range utxos {
			if util.OutPointsEqual(ut.Op, in.PreviousOutPoint) {
				u = ut
				break
			}
		}
		addr, err := w.ScriptToAddress(u.ScriptPubkey)
		if err != nil {
			return err
		}
		val, _ := strconv.ParseInt(u.Value, 10, 64)
		input := model.Input{
			Txid: in.PreviousOutPoint.Hash.String(),
			Vout: int(in.PreviousOutPoint.Index),
			ScriptSig: model.Script{
				Hex: hex.EncodeToString(in.SignatureScript),
			},
			Sequence: uint32(in.Sequence),
			N:        n,
			Addr:     addr.String(),
			Satoshis: val,
			Value:    float64(val) / util.SatoshisPerCoin(wi.BitcoinCash),
		}
		cTxn.Inputs = append(cTxn.Inputs, input)
	}
	for n, out := range tx.TxOut {
		addr, err := w.ScriptToAddress(out.PkScript)
		if err != nil {
			return err
		}
		output := model.Output{
			N: n,
			ScriptPubKey: model.OutScript{
				Script: model.Script{
					Hex: hex.EncodeToString(out.PkScript),
				},
				Addresses: []string{addr.String()},
			},
			Value: float64(float64(out.Value) / util.SatoshisPerCoin(wi.Bitcoin)),
		}
		cTxn.Outputs = append(cTxn.Outputs, output)
	}
	_, err = w.client.Broadcast(buf.Bytes())
	if err != nil {
		return err
	}
	w.ws.ProcessIncomingTransaction(cTxn)
	return nil
}

// AssociateTransactionWithOrder used for ORDER_PAYMENT message
func (w *BitcoinCashWallet) AssociateTransactionWithOrder(cb wi.TransactionCallback) {
	w.ws.InvokeTransactionListeners(cb)
}
