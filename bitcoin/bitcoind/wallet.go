package bitcoind

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcrpcclient"
	btc "github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/btcsuite/btcutil/txsort"
	"github.com/btcsuite/btcwallet/wallet/txrules"
	"github.com/op/go-logging"
	b39 "github.com/tyler-smith/go-bip39"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var log = logging.MustGetLogger("bitcoind")

const (
	Account = "OpenBazaar"
)

type BitcoindWallet struct {
	params           *chaincfg.Params
	repoPath         string
	trustedPeer      string
	masterPrivateKey *hd.ExtendedKey
	masterPublicKey  *hd.ExtendedKey
	listeners        []func(spvwallet.TransactionCallback)
	rpcClient        *btcrpcclient.Client
	binary           string
	controlPort      int
	useTor           bool
}

var connCfg *btcrpcclient.ConnConfig = &btcrpcclient.ConnConfig{
	Host:                 "localhost:8332",
	HTTPPostMode:         true, // Bitcoin core only supports HTTP POST mode
	DisableTLS:           true, // Bitcoin core does not provide TLS by default
	DisableAutoReconnect: false,
	DisableConnectOnNew:  false,
}

func NewBitcoindWallet(mnemonic string, params *chaincfg.Params, repoPath string, trustedPeer string, binary string, username string, password string, useTor bool, torControlPort int) *BitcoindWallet {
	seed := b39.NewSeed(mnemonic, "")
	mPrivKey, _ := hd.NewMaster(seed, params)
	mPubKey, _ := mPrivKey.Neuter()

	if params.Name == chaincfg.TestNet3Params.Name || params.Name == chaincfg.RegressionNetParams.Name {
		connCfg.Host = "localhost:18332"
	}

	connCfg.User = username
	connCfg.Pass = password

	if trustedPeer != "" {
		trustedPeer = strings.Split(trustedPeer, ":")[0]
	}

	w := BitcoindWallet{
		params:           params,
		repoPath:         repoPath,
		trustedPeer:      trustedPeer,
		masterPrivateKey: mPrivKey,
		masterPublicKey:  mPubKey,
		binary:           binary,
		controlPort:      torControlPort,
		useTor:           useTor,
	}
	return &w
}

func (w *BitcoindWallet) BuildArguments(rescan bool) []string {
	notify := `curl -d %s http://localhost:8330/`
	args := []string{"-walletnotify=" + notify, "-server"}
	if rescan {
		args = append(args, "-rescan")
	}
	args = append(args, "-torcontrol=127.0.0.1:"+strconv.Itoa(w.controlPort))

	if w.params.Name == chaincfg.TestNet3Params.Name {
		args = append(args, "-testnet")
	} else if w.params.Name == chaincfg.RegressionNetParams.Name {
		args = append(args, "-regtest")
	}
	if w.trustedPeer != "" {
		args = append(args, "-connect="+w.trustedPeer)
	}
	if w.useTor {
		socksPort := defaultSocksPort(w.controlPort)
		args = append(args, "-listen", "-proxy:127.0.0.1:"+strconv.Itoa(socksPort), "-onlynet=onion")
	}
	return args
}

func (w *BitcoindWallet) Start() {
	w.shutdownIfActive()
	args := w.BuildArguments(false)
	client, _ := btcrpcclient.New(connCfg, nil)
	w.rpcClient = client
	go startNotificationListener(client, w.listeners)

	cmd := exec.Command(w.binary, args...)
	cmd.Start()
	ticker := time.NewTicker(time.Second * 30)
	go func() {
		for range ticker.C {
			log.Fatal("Failed to connect to bitcoind")
		}
	}()
	for {
		_, err := client.GetBlockCount()
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	ticker.Stop()
	log.Info("Connected to bitcoind")
}

// If bitcoind is already running let's shut it down so we restart it with our options
func (w *BitcoindWallet) shutdownIfActive() {
	client, err := btcrpcclient.New(connCfg, nil)
	if err != nil {
		return
	}
	client.RawRequest("stop", []json.RawMessage{})
	client.Shutdown()
	time.Sleep(5 * time.Second)
}

func (w *BitcoindWallet) CurrencyCode() string {
	if w.params.Name == chaincfg.MainNetParams.Name {
		return "btc"
	} else {
		return "tbtc"
	}
}

func (w *BitcoindWallet) IsDust(amount int64) bool {
	return txrules.IsDustAmount(btc.Amount(amount), 25, txrules.DefaultRelayFeePerKb)
}

func (w *BitcoindWallet) MasterPrivateKey() *hd.ExtendedKey {
	return w.masterPrivateKey
}

func (w *BitcoindWallet) MasterPublicKey() *hd.ExtendedKey {
	return w.masterPublicKey
}

func (w *BitcoindWallet) CurrentAddress(purpose spvwallet.KeyPurpose) btc.Address {
	addr, _ := w.rpcClient.GetAccountAddress(Account)
	return addr
}

func (w *BitcoindWallet) NewAddress(purpose spvwallet.KeyPurpose) btc.Address {
	addr, _ := w.rpcClient.GetNewAddress(Account)
	return addr
}

func (w *BitcoindWallet) DecodeAddress(addr string) (btc.Address, error) {
	return btc.DecodeAddress(addr, w.params)
}

func (w *BitcoindWallet) ScriptToAddress(script []byte) (btc.Address, error) {
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(script, w.params)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, errors.New("unknown script")
	}
	return addrs[0], nil
}

func (w *BitcoindWallet) AddressToScript(addr btc.Address) ([]byte, error) {
	return txscript.PayToAddrScript(addr)
}

func (w *BitcoindWallet) HasKey(addr btc.Address) bool {
	_, err := w.rpcClient.DumpPrivKey(addr)
	if err != nil {
		return false
	}
	return true
}

func (w *BitcoindWallet) Balance() (confirmed, unconfirmed int64) {
	resp, _ := w.rpcClient.RawRequest("getwalletinfo", []json.RawMessage{})
	type walletInfo struct {
		Balance     float64 `json:"balance"`
		Unconfirmed float64 `json:"unconfirmed_balance"`
	}
	respBytes, _ := resp.MarshalJSON()
	i := new(walletInfo)
	json.Unmarshal(respBytes, i)
	c, _ := btc.NewAmount(i.Balance)
	u, _ := btc.NewAmount(i.Unconfirmed)
	return int64(c.ToUnit(btc.AmountSatoshi)), int64(u.ToUnit(btc.AmountSatoshi))
}

func (w *BitcoindWallet) Transactions() ([]spvwallet.Txn, error) {
	var ret []spvwallet.Txn
	resp, err := w.rpcClient.ListTransactions(Account)
	if err != nil {
		return ret, err
	}
	for _, r := range resp {
		amt, err := btc.NewAmount(r.Amount)
		if err != nil {
			return ret, err
		}
		ts := time.Unix(r.TimeReceived, 0)
		height := int32(0)
		if r.BlockIndex != nil {
			height = int32(*r.BlockIndex)
		}
		t := spvwallet.Txn{
			Txid:      r.TxID,
			Value:     int64(amt.ToUnit(btc.AmountSatoshi)),
			Height:    height,
			Timestamp: ts,
		}
		ret = append(ret, t)
	}
	return ret, nil
}

func (w *BitcoindWallet) GetTransaction(txid chainhash.Hash) (spvwallet.Txn, error) {
	includeWatchOnly := false
	t := spvwallet.Txn{}
	resp, err := w.rpcClient.GetTransaction(&txid, &includeWatchOnly)
	if err != nil {
		return t, err
	}
	t.Txid = resp.TxID
	t.Value = int64(resp.Amount * 100000000)
	t.Height = int32(resp.BlockIndex)
	t.Timestamp = time.Unix(resp.TimeReceived, 0)
	t.WatchOnly = false
	return t, nil
}

func (w *BitcoindWallet) GetConfirmations(txid chainhash.Hash) (uint32, uint32, error) {
	includeWatchOnly := true
	resp, err := w.rpcClient.GetTransaction(&txid, &includeWatchOnly)
	if err != nil {
		return 0, 0, err
	}
	return uint32(resp.Confirmations), uint32(resp.BlockIndex), nil
}

func (w *BitcoindWallet) ChainTip() uint32 {
	info, err := w.rpcClient.GetInfo()
	if err != nil {
		return uint32(0)
	}
	return uint32(info.Blocks)
}

func (w *BitcoindWallet) Spend(amount int64, addr btc.Address, feeLevel spvwallet.FeeLevel) (*chainhash.Hash, error) {
	amt, err := btc.NewAmount(float64(amount) / 100000000)
	if err != nil {
		return nil, err
	}
	return w.rpcClient.SendFrom(Account, addr, amt)
}

func (w *BitcoindWallet) BumpFee(txid chainhash.Hash) (*chainhash.Hash, error) {
	includeWatchOnly := false
	tx, err := w.rpcClient.GetTransaction(&txid, &includeWatchOnly)
	if err != nil {
		return nil, err
	}
	if tx.Confirmations > 0 {
		return nil, spvwallet.BumpFeeAlreadyConfirmedError
	}
	unspent, err := w.rpcClient.ListUnspent()
	if err != nil {
		return nil, err
	}
	for _, u := range unspent {
		if u.TxID == txid.String() {
			if u.Confirmations > 0 {
				return nil, spvwallet.BumpFeeAlreadyConfirmedError
			}
			h, err := chainhash.NewHashFromStr(u.TxID)
			if err != nil {
				continue
			}
			op := wire.NewOutPoint(h, u.Vout)
			script, err := hex.DecodeString(u.ScriptPubKey)
			if err != nil {
				continue
			}
			utxo := spvwallet.Utxo{
				Op:           *op,
				Value:        int64(u.Amount / 100000000),
				ScriptPubkey: script,
			}
			addr, err := btc.DecodeAddress(u.Address, w.params)
			if err != nil {
				continue
			}
			key, err := w.rpcClient.DumpPrivKey(addr)
			if err != nil {
				continue
			}
			hdKey := hd.NewExtendedKey(w.Params().HDPrivateKeyID[:], key.PrivKey.Serialize(), make([]byte, 32), make([]byte, 4), 0, 0, true)
			transactionID, err := w.SweepAddress([]spvwallet.Utxo{utxo}, nil, hdKey, nil, spvwallet.FEE_BUMP)
			if err != nil {
				return nil, err
			}
			return transactionID, nil

		}
	}
	return nil, spvwallet.BumpFeeNotFoundError
}

func (w *BitcoindWallet) GetFeePerByte(feeLevel spvwallet.FeeLevel) uint64 {
	defautlFee := uint64(50)
	var nBlocks json.RawMessage
	switch feeLevel {
	case spvwallet.PRIOIRTY:
		nBlocks = json.RawMessage([]byte(`1`))
	case spvwallet.NORMAL:
		nBlocks = json.RawMessage([]byte(`3`))
	case spvwallet.ECONOMIC:
		nBlocks = json.RawMessage([]byte(`6`))
	default:
		return defautlFee
	}
	resp, err := w.rpcClient.RawRequest("estimatefee", []json.RawMessage{nBlocks})
	if err != nil {
		return defautlFee
	}
	feePerKb, err := strconv.Atoi(string(resp))
	if err != nil {
		return defautlFee
	}
	if feePerKb <= 0 {
		return defautlFee
	}
	fee := feePerKb / 1000
	return uint64(fee)
}

func (w *BitcoindWallet) EstimateFee(ins []spvwallet.TransactionInput, outs []spvwallet.TransactionOutput, feePerByte uint64) uint64 {
	tx := wire.NewMsgTx(wire.TxVersion)
	for _, out := range outs {
		output := wire.NewTxOut(out.Value, out.ScriptPubKey)
		tx.TxOut = append(tx.TxOut, output)
	}
	estimatedSize := spvwallet.EstimateSerializeSize(len(ins), tx.TxOut, false)
	fee := estimatedSize * int(feePerByte)
	return uint64(fee)
}

func (w *BitcoindWallet) CreateMultisigSignature(ins []spvwallet.TransactionInput, outs []spvwallet.TransactionOutput, key *hd.ExtendedKey, redeemScript []byte, feePerByte uint64) ([]spvwallet.Signature, error) {
	var sigs []spvwallet.Signature
	tx := wire.NewMsgTx(wire.TxVersion)
	for _, in := range ins {
		ch, err := chainhash.NewHashFromStr(hex.EncodeToString(in.OutpointHash))
		if err != nil {
			return sigs, err
		}
		outpoint := wire.NewOutPoint(ch, in.OutpointIndex)
		input := wire.NewTxIn(outpoint, []byte{})
		tx.TxIn = append(tx.TxIn, input)
	}
	for _, out := range outs {
		output := wire.NewTxOut(out.Value, out.ScriptPubKey)
		tx.TxOut = append(tx.TxOut, output)
	}

	// Subtract fee
	estimatedSize := spvwallet.EstimateSerializeSize(len(ins), tx.TxOut, false)
	fee := estimatedSize * int(feePerByte)
	if len(tx.TxOut) > 0 {
		feePerOutput := fee / len(tx.TxOut)
		for _, output := range tx.TxOut {
			output.Value -= int64(feePerOutput)
		}
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	signingKey, err := key.ECPrivKey()
	if err != nil {
		return sigs, err
	}

	for i := range tx.TxIn {
		sig, err := txscript.RawTxInSignature(tx, i, redeemScript, txscript.SigHashAll, signingKey)
		if err != nil {
			continue
		}
		bs := spvwallet.Signature{InputIndex: uint32(i), Signature: sig}
		sigs = append(sigs, bs)
	}
	return sigs, nil
}

func (w *BitcoindWallet) Multisign(ins []spvwallet.TransactionInput, outs []spvwallet.TransactionOutput, sigs1 []spvwallet.Signature, sigs2 []spvwallet.Signature, redeemScript []byte, feePerByte uint64, broadcast bool) ([]byte, error) {
	tx := wire.NewMsgTx(wire.TxVersion)
	for _, in := range ins {
		ch, err := chainhash.NewHashFromStr(hex.EncodeToString(in.OutpointHash))
		if err != nil {
			return nil, err
		}
		outpoint := wire.NewOutPoint(ch, in.OutpointIndex)
		input := wire.NewTxIn(outpoint, []byte{})
		tx.TxIn = append(tx.TxIn, input)
	}
	for _, out := range outs {
		output := wire.NewTxOut(out.Value, out.ScriptPubKey)
		tx.TxOut = append(tx.TxOut, output)
	}

	// Subtract fee
	estimatedSize := spvwallet.EstimateSerializeSize(len(ins), tx.TxOut, false)
	fee := estimatedSize * int(feePerByte)
	if len(tx.TxOut) > 0 {
		feePerOutput := fee / len(tx.TxOut)
		for _, output := range tx.TxOut {
			output.Value -= int64(feePerOutput)
		}
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	for i, input := range tx.TxIn {
		var sig1 []byte
		var sig2 []byte
		for _, sig := range sigs1 {
			if int(sig.InputIndex) == i {
				sig1 = sig.Signature
			}
		}
		for _, sig := range sigs2 {
			if int(sig.InputIndex) == i {
				sig2 = sig.Signature
			}
		}
		builder := txscript.NewScriptBuilder()
		builder.AddOp(txscript.OP_0)
		builder.AddData(sig1)
		builder.AddData(sig2)
		builder.AddData(redeemScript)
		scriptSig, err := builder.Script()
		if err != nil {
			return nil, err
		}
		input.SignatureScript = scriptSig
	}
	// Broadcast
	if broadcast {
		_, err := w.rpcClient.SendRawTransaction(tx, false)
		if err != nil {
			return nil, err
		}
	}
	var buf bytes.Buffer
	tx.BtcEncode(&buf, 1)
	return buf.Bytes(), nil
}

func (w *BitcoindWallet) SweepAddress(utxos []spvwallet.Utxo, address *btc.Address, key *hd.ExtendedKey, redeemScript *[]byte, feeLevel spvwallet.FeeLevel) (*chainhash.Hash, error) {
	var internalAddr btc.Address
	if address != nil {
		internalAddr = *address
	} else {
		internalAddr = w.CurrentAddress(spvwallet.INTERNAL)
	}
	script, err := txscript.PayToAddrScript(internalAddr)
	if err != nil {
		return nil, err
	}

	var val int64
	var inputs []*wire.TxIn
	additionalPrevScripts := make(map[wire.OutPoint][]byte)
	for _, u := range utxos {
		val += u.Value
		in := wire.NewTxIn(&u.Op, []byte{})
		inputs = append(inputs, in)
		additionalPrevScripts[u.Op] = u.ScriptPubkey
	}
	out := wire.NewTxOut(val, script)

	estimatedSize := spvwallet.EstimateSerializeSize(len(utxos), []*wire.TxOut{out}, false)

	// Calculate the fee
	b := json.RawMessage([]byte(`1`))
	resp, err := w.rpcClient.RawRequest("estimatefee", []json.RawMessage{b})
	if err != nil {
		return nil, err
	}
	feePerKb, err := strconv.Atoi(string(resp))
	if err != nil {
		return nil, err
	}
	if feePerKb <= 0 {
		feePerKb = 50000
	}
	fee := estimatedSize * (feePerKb / 1000)

	outVal := val - int64(fee)
	if outVal < 0 {
		outVal = 0
	}
	out.Value = outVal

	tx := &wire.MsgTx{
		Version:  wire.TxVersion,
		TxIn:     inputs,
		TxOut:    []*wire.TxOut{out},
		LockTime: 0,
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	// Sign tx
	getKey := txscript.KeyClosure(func(addr btc.Address) (*btcec.PrivateKey, bool, error) {
		privKey, err := key.ECPrivKey()
		if err != nil {
			return nil, false, err
		}
		wif, err := btc.NewWIF(privKey, w.params, true)
		if err != nil {
			return nil, false, err
		}
		return wif.PrivKey, wif.CompressPubKey, nil
	})
	getScript := txscript.ScriptClosure(func(addr btc.Address) ([]byte, error) {
		if redeemScript == nil {
			return []byte{}, nil
		}
		return *redeemScript, nil
	})

	for i, txIn := range tx.TxIn {
		prevOutScript := additionalPrevScripts[txIn.PreviousOutPoint]
		script, err := txscript.SignTxOutput(w.params,
			tx, i, prevOutScript, txscript.SigHashAll, getKey,
			getScript, txIn.SignatureScript)
		if err != nil {
			return nil, errors.New("Failed to sign transaction")
		}
		txIn.SignatureScript = script
	}

	// Broadcast
	_, err = w.rpcClient.SendRawTransaction(tx, false)
	if err != nil {
		return nil, err
	}
	txid := tx.TxHash()
	return &txid, nil
}

func (w *BitcoindWallet) Params() *chaincfg.Params {
	return w.params
}

func (w *BitcoindWallet) AddTransactionListener(callback func(spvwallet.TransactionCallback)) {
	w.listeners = append(w.listeners, callback)
}

func (w *BitcoindWallet) GenerateMultisigScript(keys []hd.ExtendedKey, threshold int) (addr btc.Address, redeemScript []byte, err error) {
	var addrPubKeys []*btc.AddressPubKey
	for _, key := range keys {
		ecKey, err := key.ECPubKey()
		if err != nil {
			return nil, nil, err
		}
		k, err := btc.NewAddressPubKey(ecKey.SerializeCompressed(), w.params)
		if err != nil {
			return nil, nil, err
		}
		addrPubKeys = append(addrPubKeys, k)
	}
	redeemScript, err = txscript.MultiSigScript(addrPubKeys, threshold)
	if err != nil {
		return nil, nil, err
	}
	addr, err = btc.NewAddressScriptHash(redeemScript, w.params)
	if err != nil {
		return nil, nil, err
	}
	return addr, redeemScript, nil
}

func (w *BitcoindWallet) AddWatchedScript(script []byte) error {
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(script, w.params)
	if err != nil {
		return err
	}
	return w.rpcClient.ImportAddress(addrs[0].EncodeAddress())
}

func (w *BitcoindWallet) ReSyncBlockchain(fromHeight int32) {
	w.rpcClient.RawRequest("stop", []json.RawMessage{})
	w.rpcClient.Shutdown()
	time.Sleep(5 * time.Second)
	args := w.BuildArguments(true)
	cmd := exec.Command(w.binary, args...)
	cmd.Start()

	client, err := btcrpcclient.New(connCfg, nil)
	if err != nil {
		log.Error("Could not connect to bitcoind during rescan")
	}
	w.rpcClient = client
}

func (w *BitcoindWallet) Close() {
	if w.rpcClient != nil {
		w.rpcClient.RawRequest("stop", []json.RawMessage{})
		w.rpcClient.Shutdown()
	}
}

func defaultSocksPort(controlPort int) int {
	socksPort := 9050
	if controlPort == 9151 || controlPort == 9051 {
		controlPort--
		socksPort = controlPort
	}
	return socksPort
}
