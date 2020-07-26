package filecoin

import (
	"fmt"
	"github.com/OpenBazaar/multiwallet/keys"
	"github.com/btcsuite/btcd/btcec"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/lib/sigs"
	_ "github.com/filecoin-project/lotus/lib/sigs/secp"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"io"
	"math/big"
	"time"

	"github.com/OpenBazaar/multiwallet/cache"
	"github.com/OpenBazaar/multiwallet/client"
	"github.com/OpenBazaar/multiwallet/config"
	"github.com/OpenBazaar/multiwallet/model"
	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	faddr "github.com/filecoin-project/go-address"
	"github.com/op/go-logging"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/net/proxy"
)

type FilecoinWallet struct {
	db     wi.Datastore
	params *chaincfg.Params
	client model.APIClient

	mPrivKey *hd.ExtendedKey
	mPubKey  *hd.ExtendedKey

	addr faddr.Address
	key  *btcec.PrivateKey

	fs *FilecoinService

	exchangeRates wi.ExchangeRates
	log           *logging.Logger
}

var (
	_                          = wi.Wallet(&FilecoinWallet{})
	FilecoinCurrencyDefinition = wi.CurrencyDefinition{
		Code:         "FIL",
		Divisibility: 18,
	}
)

func NewFilecoinWallet(cfg config.CoinConfig, mnemonic string, params *chaincfg.Params, proxy proxy.Dialer, cache cache.Cacher, disableExchangeRates bool) (*FilecoinWallet, error) {
	seed := bip39.NewSeed(mnemonic, "")

	mPrivKey, err := hd.NewMaster(seed, params)
	if err != nil {
		return nil, err
	}
	mPubKey, err := mPrivKey.Neuter()
	if err != nil {
		return nil, err
	}

	_, external, err := keys.Bip44Derivation(mPrivKey, wi.Filecoin)
	if err != nil {
		return nil, err
	}

	accountHDKey, err := external.Child(0)
	if err != nil {
		return nil, err
	}

	accountECKey, err := accountHDKey.ECPrivKey()
	if err != nil {
		return nil, err
	}

	accountAddr, err := faddr.NewSecp256k1Address(accountECKey.PubKey().SerializeUncompressed())
	if err != nil {
		return nil, err
	}

	c, err := client.NewClientPool(cfg.ClientAPIs, proxy)
	if err != nil {
		return nil, err
	}

	fs, err := NewFilecoinService(cfg.DB, &FilecoinAddress{addr: accountAddr}, c, params, wi.Filecoin, cache)
	if err != nil {
		return nil, err
	}

	return &FilecoinWallet{
		db:       cfg.DB,
		params:   params,
		client:   c,
		addr:     accountAddr,
		mPrivKey: mPrivKey,
		mPubKey:  mPubKey,
		key:      accountECKey,
		fs:       fs,
		log:      logging.MustGetLogger("litecoin-wallet"),
	}, nil
}

func (w *FilecoinWallet) Start() {
	w.client.Start()
	w.fs.Start()
}

func (w *FilecoinWallet) Params() *chaincfg.Params {
	return w.params
}

func (w *FilecoinWallet) CurrencyCode() string {
	if w.params.Name == chaincfg.MainNetParams.Name {
		return "fil"
	} else {
		return "tfil"
	}
}

func (w *FilecoinWallet) IsDust(amount big.Int) bool {
	// TODO
	return false
}

func (w *FilecoinWallet) MasterPrivateKey() *hd.ExtendedKey {
	return w.mPrivKey
}

func (w *FilecoinWallet) MasterPublicKey() *hd.ExtendedKey {
	return w.mPubKey
}

func (wallet *FilecoinWallet) BumpFee(txid string) (string, error) {
	return txid, nil
}

func (wallet *FilecoinWallet) CreateMultisigSignature(ins []wi.TransactionInput, outs []wi.TransactionOutput, key *hd.ExtendedKey, redeemScript []byte, feePerByte big.Int) ([]wi.Signature, error) {
	return nil, nil
}

func (w *FilecoinWallet) GenerateMultisigScript(keys []hd.ExtendedKey, threshold int, timeout time.Duration, timeoutKey *hd.ExtendedKey) (addr btcutil.Address, redeemScript []byte, err error) {
	return nil, nil, nil
}

func (wallet *FilecoinWallet) Multisign(ins []wi.TransactionInput, outs []wi.TransactionOutput, sigs1 []wi.Signature, sigs2 []wi.Signature, redeemScript []byte, feePerByte big.Int, broadcast bool) ([]byte, error) {
	return nil, nil
}

func (wallet *FilecoinWallet) SweepAddress(utxos []wi.TransactionInput, address *btcutil.Address, key *hd.ExtendedKey, redeemScript *[]byte, feeLevel wi.FeeLevel) (string, error) {
	return "", nil
}

func (w *FilecoinWallet) ChildKey(keyBytes []byte, chaincode []byte, isPrivateKey bool) (*hd.ExtendedKey, error) {
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

func (w *FilecoinWallet) CurrentAddress(purpose wi.KeyPurpose) btcutil.Address {
	return &FilecoinAddress{addr: w.addr}
}

func (w *FilecoinWallet) NewAddress(purpose wi.KeyPurpose) btcutil.Address {
	return &FilecoinAddress{addr: w.addr}
}

func (w *FilecoinWallet) DecodeAddress(addr string) (btcutil.Address, error) {
	a, err := faddr.NewFromString(addr)
	if err != nil {
		return nil, err
	}
	return &FilecoinAddress{addr: a}, nil
}

func (w *FilecoinWallet) ScriptToAddress(script []byte) (btcutil.Address, error) {
	return w.DecodeAddress(string(script))
}

func (w *FilecoinWallet) AddressToScript(addr btcutil.Address) ([]byte, error) {
	return []byte(addr.String()), nil
}

func (w *FilecoinWallet) HasKey(addr btcutil.Address) bool {
	return w.addr.String() == addr.String()
}

func (w *FilecoinWallet) Balance() (wi.CurrencyValue, wi.CurrencyValue) {
	txns, _ := w.db.Txns().GetAll(false)
	confirmed, unconfirmed := big.NewInt(0), big.NewInt(0)
	for _, tx := range txns {
		val, _ := new(big.Int).SetString(tx.Value, 10)
		if val.Cmp(big.NewInt(0)) > 0 {
			if tx.Height > 0 {
				confirmed.Add(confirmed, val)
			} else {
				unconfirmed.Add(confirmed, val)
			}
		} else if val.Cmp(big.NewInt(0)) < 0 {
			if tx.Height > 0 {
				confirmed.Sub(confirmed, val)
			} else {
				unconfirmed.Sub(confirmed, val)
			}
		}
	}
	return wi.CurrencyValue{Value: *confirmed, Currency: FilecoinCurrencyDefinition},
		wi.CurrencyValue{Value: *unconfirmed, Currency: FilecoinCurrencyDefinition}
}

func (w *FilecoinWallet) Transactions() ([]wi.Txn, error) {
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
		case confs > 0 && confs < 24:
			status = wi.StatusPending
			confirmations = confs
		case confs > 23:
			status = wi.StatusConfirmed
			confirmations = confs
		}
		tx.Confirmations = int64(confirmations)
		tx.Status = status
		txns[i] = tx
	}
	return txns, nil
}

func (w *FilecoinWallet) GetTransaction(txid string) (wi.Txn, error) {
	txn, err := w.db.Txns().Get(txid)
	return txn, err
}

func (w *FilecoinWallet) ChainTip() (uint32, string) {
	return w.fs.ChainTip()
}

func (w *FilecoinWallet) GetFeePerByte(feeLevel wi.FeeLevel) big.Int {
	return *big.NewInt(0)
}

func (w *FilecoinWallet) Spend(amount big.Int, addr btcutil.Address, feeLevel wi.FeeLevel, referenceID string, spendAll bool) (string, error) {
	address, err := faddr.NewFromString(addr.String())
	if err != nil {
		return "", err
	}
	if spendAll {
		c, u := w.Balance()
		amount = *c.Value.Add(big.NewInt(0), &u.Value)
	}
	bigAmt, err := types.BigFromString(amount.String())
	if err != nil {
		return "", err
	}

	txns, err := w.Transactions()
	if err != nil {
		return "", err
	}

	nonce := uint64(1)
	for _, tx := range txns {
		val, _ := new(big.Int).SetString(tx.Value, 10)
		if val.Cmp(big.NewInt(0)) > 0 {
			continue
		}

		m, err := types.DecodeMessage(tx.Bytes)
		if err != nil {
			return "", err
		}
		if m.Nonce > nonce {
			nonce = m.Nonce
		}
	}
	if nonce > 0 {
		nonce++
	}

	m := types.Message{
		To:       address,
		Value:    bigAmt,
		From:     w.addr,
		GasLimit: 1000,
		Nonce:    nonce,
	}

	id := m.Cid()

	cs, err := sigs.Sign(crypto.SigTypeSecp256k1, w.key.Serialize(), id.Bytes())
	if err != nil {
		return "", err
	}

	signed := &types.SignedMessage{
		Message:   m,
		Signature: *cs,
	}

	// Broadcast
	if err := w.Broadcast(signed); err != nil {
		return "", err
	}

	myAddr, err :=  NewFilecoinAddress(w.addr.String())
	if err != nil {
		return "", err
	}
	w.AssociateTransactionWithOrder(wi.TransactionCallback{
		Timestamp: time.Now(),
		Outputs: []wi.TransactionOutput{
			{
				Address: addr,
				Value: amount,
				OrderID: referenceID,
			},
		},
		Inputs: []wi.TransactionInput{
			{
				Value: amount,
				OrderID: referenceID,
				LinkedAddress: myAddr,
			},
		},
		Value: *amount.Mul(&amount, big.NewInt(-1)),
		Txid: id.String(),
	})

	return id.String(), nil
}

func (w *FilecoinWallet) EstimateFee(ins []wi.TransactionInput, outs []wi.TransactionOutput, feePerByte big.Int) big.Int {
	return *big.NewInt(1000) // TODO: find right fee
}

func (w *FilecoinWallet) EstimateSpendFee(amount big.Int, feeLevel wi.FeeLevel) (big.Int, error) {
	return *big.NewInt(1000), nil // TODO: find right fee
}

func (w *FilecoinWallet) AddWatchedAddresses(addrs ...btcutil.Address) error {
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

func (w *FilecoinWallet) AddWatchedScript(script []byte) error {
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

func (w *FilecoinWallet) AddTransactionListener(callback func(wi.TransactionCallback)) {
	w.fs.AddTransactionListener(callback)
}

func (w *FilecoinWallet) ReSyncBlockchain(fromTime time.Time) {
	go w.fs.UpdateState()
}

func (w *FilecoinWallet) GetConfirmations(txid string) (uint32, uint32, error) {
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

func (w *FilecoinWallet) Close() {
	w.fs.Stop()
	w.client.Close()
}

func (w *FilecoinWallet) ExchangeRates() wi.ExchangeRates {
	return w.exchangeRates
}

func (w *FilecoinWallet) DumpTables(wr io.Writer) {
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
	fmt.Fprintln(wr, "\nKeys-----")
	keys, _ := w.db.Keys().GetAll()
	unusedInternal, _ := w.db.Keys().GetUnused(wi.INTERNAL)
	unusedExternal, _ := w.db.Keys().GetUnused(wi.EXTERNAL)
	internalMap := make(map[int]bool)
	externalMap := make(map[int]bool)
	for _, k := range unusedInternal {
		internalMap[k] = true
	}
	for _, k := range unusedExternal {
		externalMap[k] = true
	}

	for _, k := range keys {
		var used bool
		if k.Purpose == wi.INTERNAL {
			used = internalMap[k.Index]
		} else {
			used = externalMap[k.Index]
		}
		fmt.Fprintf(wr, "KeyIndex: %d, Purpose: %d, Used: %t\n", k.Index, k.Purpose, used)
	}
}

// Build a client.Transaction so we can ingest it into the wallet service then broadcast
func (w *FilecoinWallet) Broadcast(msg *types.SignedMessage) error {
	id := msg.Cid()
	ser, err := msg.Serialize()
	if err != nil {
		return err
	}

	cTxn := model.Transaction{
		Txid:          id.String(),
		Version:       int(msg.Message.Version),
		Confirmations: 0,
		Time:          time.Now().Unix(),
		RawBytes:      ser,
	}
	output := model.Output{
		ScriptPubKey: model.OutScript{
			Addresses: []string{msg.Message.To.String()},
		},
		ValueIface: msg.Message.Value.String(),
	}
	cTxn.Outputs = append(cTxn.Outputs, output)

	_, err = w.client.Broadcast(ser)
	if err != nil {
		return err
	}
	w.fs.ProcessIncomingTransaction(cTxn)
	return nil
}

// AssociateTransactionWithOrder used for ORDER_PAYMENT message
func (w *FilecoinWallet) AssociateTransactionWithOrder(cb wi.TransactionCallback) {
	w.fs.InvokeTransactionListeners(cb)
}
