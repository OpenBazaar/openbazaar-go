package bitcoind

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/OpenBazaar/spvwallet"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	btcrpcclient "github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	btc "github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/coinset"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/btcsuite/btcutil/txsort"
	"github.com/btcsuite/btcwallet/wallet/txrules"
	"github.com/op/go-logging"
	b39 "github.com/tyler-smith/go-bip39"
	"os"
	"os/exec"
	"path"
	"runtime"
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
	listeners        []func(wallet.TransactionCallback)
	rpcClient        *btcrpcclient.Client
	binary           string
	controlPort      int
	useTor           bool
	addrsToWatch     []btc.Address
	initChan         chan struct{}
}

var connCfg *btcrpcclient.ConnConfig = &btcrpcclient.ConnConfig{
	Host:                 "localhost:8332",
	HTTPPostMode:         true, // Bitcoin core only supports HTTP POST mode
	DisableTLS:           true, // Bitcoin core does not provide TLS by default
	DisableAutoReconnect: false,
	DisableConnectOnNew:  false,
}

func NewBitcoindWallet(mnemonic string, params *chaincfg.Params, repoPath string, trustedPeer string, binary string, useTor bool, torControlPort int) (*BitcoindWallet, error) {
	seed := b39.NewSeed(mnemonic, "")
	mPrivKey, _ := hd.NewMaster(seed, params)
	mPubKey, _ := mPrivKey.Neuter()

	if params.Name == chaincfg.TestNet3Params.Name || params.Name == chaincfg.RegressionNetParams.Name {
		connCfg.Host = "localhost:18332"
	}

	dataDir := path.Join(repoPath, "zcash")
	var err error
	connCfg.User, connCfg.Pass, err = GetCredentials(repoPath)
	if err != nil {
		return nil, err
	}

	if trustedPeer != "" {
		trustedPeer = strings.Split(trustedPeer, ":")[0]
	}

	w := BitcoindWallet{
		params:           params,
		repoPath:         dataDir,
		trustedPeer:      trustedPeer,
		masterPrivateKey: mPrivKey,
		masterPublicKey:  mPubKey,
		binary:           binary,
		controlPort:      torControlPort,
		useTor:           useTor,
		initChan:         make(chan struct{}),
	}
	return &w, nil
}

// TestNetworkEnabled indicates if the current network being used is Test Network
func (w *BitcoindWallet) TestNetworkEnabled() bool {
	return w.params.Name == chaincfg.TestNet3Params.Name
}

// RegressionNetworkEnabled indicates if the current network being used is Regression Network
func (w *BitcoindWallet) RegressionNetworkEnabled() bool {
	return w.params.Name == chaincfg.RegressionNetParams.Name
}

// MainNetworkEnabled indicates if the current network being used is the live Network
func (w *BitcoindWallet) MainNetworkEnabled() bool {
	return w.params.Name == chaincfg.MainNetParams.Name
}

func GetCredentials(repoPath string) (username, password string, err error) {
	p := path.Join(repoPath, "bitcoin", "bitcoin.conf")
	if _, err := os.Stat(p); os.IsNotExist(err) {
		dataDir := path.Join(repoPath, "bitcoin")
		os.Mkdir(dataDir, os.ModePerm)

		r := make([]byte, 32)
		_, err := rand.Read(r)
		if err != nil {
			return "", "", err
		}
		password := base64.StdEncoding.EncodeToString(r)

		user := fmt.Sprintf(`rpcuser=%s`, "OpenBazaar")
		pass := fmt.Sprintf(`rpcpassword=%s`, password)

		f, err := os.Create(p)
		if err != nil {
			return "", "", err
		}
		defer f.Close()
		wr := bufio.NewWriter(f)
		fmt.Fprintln(wr, user)
		fmt.Fprintln(wr, pass)
		wr.Flush()
		return "OpenBazaar", password, nil
	} else {
		file, err := os.Open(p)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		var unExists, pwExists bool
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "rpcuser=") {
				username = scanner.Text()[8:]
				unExists = true
			} else if strings.Contains(scanner.Text(), "rpcpassword=") {
				password = scanner.Text()[12:]
				pwExists = true
			}
		}
		if !unExists || !pwExists {
			return "", "", errors.New("Bitcoin config file does not contain a username and password")
		}

		if err := scanner.Err(); err != nil {
			return "", "", err
		}
		return username, password, nil
	}
}

func (w *BitcoindWallet) addQueuedWatchAddresses() {
	for _, addr := range w.addrsToWatch {
		w.addWatchedScript(addr)
	}
}

func (w *BitcoindWallet) InitChan() chan struct{} {
	return w.initChan
}

func (w *BitcoindWallet) BuildArguments(rescan bool) []string {
	var notify string
	switch runtime.GOOS {
	case "windows":
		notify = `powershell.exe Invoke-WebRequest -Uri http://localhost:8330/ -Method POST -Body %s`
	default:
		notify = `curl -d %s http://localhost:8330/`
	}
	args := []string{"-walletnotify=" + notify, "-server", "-wallet=ob-wallet.dat", "-conf=" + path.Join(w.repoPath, "bitcoin.conf")}
	if rescan {
		args = append(args, "-rescan")
	}
	args = append(args, "-torcontrol=127.0.0.1:"+strconv.Itoa(w.controlPort))

	if w.TestNetworkEnabled() {
		args = append(args, "-testnet")
	} else if w.RegressionNetworkEnabled() {
		args = append(args, "-regtest")
	}
	if w.trustedPeer != "" {
		args = append(args, "-connect="+w.trustedPeer)
	}
	if w.useTor {
		socksPort := DefaultSocksPort(w.controlPort)
		args = append(args, "-listen", "-proxy:127.0.0.1:"+strconv.Itoa(socksPort), "-onlynet=onion")
	}
	return args
}

func (w *BitcoindWallet) Start() {
	w.shutdownIfActive()
	args := w.BuildArguments(false)
	client, _ := btcrpcclient.New(connCfg, nil)
	w.rpcClient = client
	go StartNotificationListener(client, w.params, w.listeners)

	cmd := exec.Command(w.binary, args...)
	go cmd.Start()
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
	close(w.initChan)
	go w.addQueuedWatchAddresses()
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
	if w.MainNetworkEnabled() {
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

func (w *BitcoindWallet) ChildKey(keyBytes []byte, chaincode []byte, isPrivateKey bool) (*hd.ExtendedKey, error) {
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

func (w *BitcoindWallet) CurrentAddress(purpose wallet.KeyPurpose) btc.Address {
	<-w.initChan
	addr, _ := w.rpcClient.GetAccountAddress(Account)
	return addr
}

func (w *BitcoindWallet) NewAddress(purpose wallet.KeyPurpose) btc.Address {
	<-w.initChan
	addr, _ := w.rpcClient.GetNewAddress(Account)
	return addr
}

func (w *BitcoindWallet) DecodeAddress(addr string) (btc.Address, error) {
	return btc.DecodeAddress(addr, w.params)
}

func (w *BitcoindWallet) ScriptToAddress(script []byte) (btc.Address, error) {
	return scriptToAddress(script, w.params)
}

func scriptToAddress(script []byte, params *chaincfg.Params) (btc.Address, error) {
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(script, params)
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
	<-w.initChan
	_, err := w.rpcClient.DumpPrivKey(addr)
	if err != nil {
		return false
	}
	return true
}

func (w *BitcoindWallet) Balance() (confirmed, unconfirmed int64) {
	<-w.initChan
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

func (w *BitcoindWallet) GetBlockHeight(hash *chainhash.Hash) (int32, error) {
	<-w.initChan
	blockinfo, err := w.rpcClient.GetBlockHeaderVerbose(hash)
	if err != nil {
		return 0, err
	}
	return blockinfo.Height, nil
}

func (w *BitcoindWallet) Transactions() ([]wallet.Txn, error) {
	<-w.initChan
	var ret []wallet.Txn
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
		if r.Confirmations > 0 {
			h, err := chainhash.NewHashFromStr(r.BlockHash)
			if err != nil {
				return ret, err
			}
			height, err = w.GetBlockHeight(h)
			if err != nil {
				return ret, err
			}
		}

		var confirmations int32
		var status wallet.StatusCode
		confs := int32(height) - height + 1
		if height <= 0 {
			confs = height
		}
		switch {
		case confs < 0:
			status = wallet.StatusDead
		case confs == 0 && time.Since(ts) <= time.Hour*6:
			status = wallet.StatusUnconfirmed
		case confs == 0 && time.Since(ts) > time.Hour*6:
			status = wallet.StatusStuck
		case confs > 0 && confs < 6:
			status = wallet.StatusPending
			confirmations = confs
		case confs > 5:
			status = wallet.StatusConfirmed
			confirmations = confs
		}

		t := wallet.Txn{
			Txid:          r.TxID,
			Value:         int64(amt.ToUnit(btc.AmountSatoshi)),
			Height:        height,
			Timestamp:     ts,
			Confirmations: int64(confirmations),
			Status:        status,
		}
		ret = append(ret, t)
	}
	return ret, nil
}

func (w *BitcoindWallet) GetTransaction(txid chainhash.Hash) (wallet.Txn, error) {
	<-w.initChan
	includeWatchOnly := false
	t := wallet.Txn{}
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
	<-w.initChan
	includeWatchOnly := true
	resp, err := w.rpcClient.GetTransaction(&txid, &includeWatchOnly)
	if err != nil {
		return 0, 0, err
	}
	return uint32(resp.Confirmations), uint32(resp.BlockIndex), nil
}

func (w *BitcoindWallet) ChainTip() (uint32, chainhash.Hash) {
	<-w.initChan
	var ch chainhash.Hash
	info, err := w.rpcClient.GetInfo()
	if err != nil {
		return uint32(0), ch
	}
	h, err := w.rpcClient.GetBestBlockHash()
	if err != nil {
		return uint32(0), ch
	}
	return uint32(info.Blocks), *h
}

func (w *BitcoindWallet) gatherCoins() (map[coinset.Coin]*hd.ExtendedKey, error) {
	<-w.initChan
	m := make(map[coinset.Coin]*hd.ExtendedKey)
	utxos, err := w.rpcClient.ListUnspent()
	if err != nil {
		return m, err
	}
	for _, u := range utxos {
		if !u.Spendable {
			continue
		}
		txhash, err := chainhash.NewHashFromStr(u.TxID)
		if err != nil {
			return m, err
		}
		addr, err := btc.DecodeAddress(u.Address, w.params)
		if err != nil {
			return m, err
		}
		scriptPubkey, err := w.AddressToScript(addr)
		if err != nil {
			return m, err
		}
		c := spvwallet.NewCoin(txhash.CloneBytes(), u.Vout, btc.Amount(u.Amount*100000000), u.Confirmations, scriptPubkey)

		wif, err := w.rpcClient.DumpPrivKey(addr)
		if err != nil {
			return m, err
		}
		key := hd.NewExtendedKey(
			w.params.HDPrivateKeyID[:],
			wif.PrivKey.Serialize(),
			make([]byte, 32),
			[]byte{0x00, 0x00, 0x00, 0x00},
			0,
			0,
			true)
		m[c] = key
	}
	return m, nil
}

func (w *BitcoindWallet) Spend(amount int64, addr btc.Address, feeLevel wallet.FeeLevel) (*chainhash.Hash, error) {
	<-w.initChan
	tx, err := w.buildTx(amount, addr, feeLevel)
	if err != nil {
		return nil, err
	}
	return w.rpcClient.SendRawTransaction(tx, false)
}

func (w *BitcoindWallet) buildTx(amount int64, addr btc.Address, feeLevel wallet.FeeLevel) (*wire.MsgTx, error) {
	script, _ := txscript.PayToAddrScript(addr)
	if txrules.IsDustAmount(btc.Amount(amount), len(script), txrules.DefaultRelayFeePerKb) {
		return nil, wallet.ErrorDustAmount
	}

	var additionalPrevScripts map[wire.OutPoint][]byte
	var additionalKeysByAddress map[string]*btc.WIF

	// Create input source
	coinMap, err := w.gatherCoins()
	if err != nil {
		return nil, err
	}
	coins := make([]coinset.Coin, 0, len(coinMap))
	for k := range coinMap {
		coins = append(coins, k)
	}
	inputSource := func(target btc.Amount) (total btc.Amount, inputs []*wire.TxIn, amounts []btc.Amount, scripts [][]byte, err error) {
		coinSelector := coinset.MaxValueAgeCoinSelector{MaxInputs: 10000, MinChangeAmount: btc.Amount(0)}
		coins, err := coinSelector.CoinSelect(target, coins)
		if err != nil {
			return total, inputs, []btc.Amount{}, scripts, wallet.ErrorInsuffientFunds
		}
		additionalPrevScripts = make(map[wire.OutPoint][]byte)
		additionalKeysByAddress = make(map[string]*btc.WIF)
		for _, c := range coins.Coins() {
			total += c.Value()
			outpoint := wire.NewOutPoint(c.Hash(), c.Index())
			in := wire.NewTxIn(outpoint, []byte{}, [][]byte{})
			in.Sequence = 0 // Opt-in RBF so we can bump fees
			inputs = append(inputs, in)
			additionalPrevScripts[*outpoint] = c.PkScript()
			key := coinMap[c]
			addr, err := key.Address(w.params)
			if err != nil {
				continue
			}
			privKey, err := key.ECPrivKey()
			if err != nil {
				continue
			}
			wif, _ := btc.NewWIF(privKey, w.params, true)
			additionalKeysByAddress[addr.EncodeAddress()] = wif
		}
		return total, inputs, []btc.Amount{}, scripts, nil
	}

	// Get the fee per kilobyte
	feePerKB := int64(w.GetFeePerByte(feeLevel)) * 1000

	// outputs
	out := wire.NewTxOut(amount, script)

	// Create change source
	changeSource := func() ([]byte, error) {
		addr := w.CurrentAddress(wallet.INTERNAL)
		script, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return []byte{}, err
		}
		return script, nil
	}

	outputs := []*wire.TxOut{out}
	authoredTx, err := spvwallet.NewUnsignedTransaction(outputs, btc.Amount(feePerKB), inputSource, changeSource)
	if err != nil {
		return nil, err
	}

	// BIP 69 sorting
	txsort.InPlaceSort(authoredTx.Tx)

	// Sign tx
	getKey := txscript.KeyClosure(func(addr btc.Address) (*btcec.PrivateKey, bool, error) {
		addrStr := addr.EncodeAddress()
		wif := additionalKeysByAddress[addrStr]
		return wif.PrivKey, wif.CompressPubKey, nil
	})
	getScript := txscript.ScriptClosure(func(
		addr btc.Address) ([]byte, error) {
		return []byte{}, nil
	})
	for i, txIn := range authoredTx.Tx.TxIn {
		prevOutScript := additionalPrevScripts[txIn.PreviousOutPoint]
		script, err := txscript.SignTxOutput(w.params,
			authoredTx.Tx, i, prevOutScript, txscript.SigHashAll, getKey,
			getScript, txIn.SignatureScript)
		if err != nil {
			return nil, errors.New("Failed to sign transaction")
		}
		txIn.SignatureScript = script
	}
	return authoredTx.Tx, nil
}

func (w *BitcoindWallet) BumpFee(txid chainhash.Hash) (*chainhash.Hash, error) {
	<-w.initChan
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
			addr, err := btc.DecodeAddress(u.Address, w.params)
			if err != nil {
				continue
			}
			key, err := w.rpcClient.DumpPrivKey(addr)
			if err != nil {
				continue
			}
			in := wallet.TransactionInput{
				LinkedAddress: addr,
				OutpointIndex: u.Vout,
				OutpointHash:  h.CloneBytes(),
				Value:         int64(u.Amount),
			}
			hdKey := hd.NewExtendedKey(w.params.HDPrivateKeyID[:], key.PrivKey.Serialize(), make([]byte, 32), make([]byte, 4), 0, 0, true)
			transactionID, err := w.SweepAddress([]wallet.TransactionInput{in}, nil, hdKey, nil, wallet.FEE_BUMP)
			if err != nil {
				return nil, err
			}
			return transactionID, nil
		}
	}
	return nil, spvwallet.BumpFeeNotFoundError
}

func (w *BitcoindWallet) GetFeePerByte(feeLevel wallet.FeeLevel) uint64 {
	<-w.initChan
	defautlFee := uint64(50)
	var nBlocks json.RawMessage
	switch feeLevel {
	case wallet.PRIOIRTY:
		nBlocks = json.RawMessage([]byte(`1`))
	case wallet.NORMAL:
		nBlocks = json.RawMessage([]byte(`3`))
	case wallet.ECONOMIC:
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

func (w *BitcoindWallet) EstimateFee(ins []wallet.TransactionInput, outs []wallet.TransactionOutput, feePerByte uint64) uint64 {
	tx := wire.NewMsgTx(wire.TxVersion)
	for _, out := range outs {
		scriptPubKey, _ := txscript.PayToAddrScript(out.Address)
		output := wire.NewTxOut(out.Value, scriptPubKey)
		tx.TxOut = append(tx.TxOut, output)
	}
	estimatedSize := spvwallet.EstimateSerializeSize(len(ins), tx.TxOut, false, spvwallet.P2PKH)
	fee := estimatedSize * int(feePerByte)
	return uint64(fee)
}

func (w *BitcoindWallet) EstimateSpendFee(amount int64, feeLevel wallet.FeeLevel) (uint64, error) {
	<-w.initChan
	// Since this is an estimate we can use a dummy output address. Let's use a long one so we don't under estimate.
	addr, err := btc.DecodeAddress("bc1qxtq7ha2l5qg70atpwp3fus84fx3w0v2w4r2my7gt89ll3w0vnlgspu349h", &chaincfg.MainNetParams)
	if err != nil {
		return 0, err
	}
	tx, err := w.buildTx(amount, addr, feeLevel)
	if err != nil {
		return 0, err
	}
	var outval int64
	for _, output := range tx.TxOut {
		outval += output.Value
	}
	var inval int64
	utxos, err := w.rpcClient.ListUnspent()
	if err != nil {
		return 0, err
	}
	for _, input := range tx.TxIn {
		for _, utxo := range utxos {
			if utxo.TxID == input.PreviousOutPoint.Hash.String() && utxo.Vout == input.PreviousOutPoint.Index {
				inval += int64(utxo.Amount * 100000000)
				break
			}
		}
	}
	if inval < outval {
		return 0, errors.New("Error building transaction: inputs less than outputs")
	}
	return uint64(inval - outval), err
}

func (w *BitcoindWallet) CreateMultisigSignature(ins []wallet.TransactionInput, outs []wallet.TransactionOutput, key *hd.ExtendedKey, redeemScript []byte, feePerByte uint64) ([]wallet.Signature, error) {
	var sigs []wallet.Signature
	tx := wire.NewMsgTx(1)
	for _, in := range ins {
		ch, err := chainhash.NewHashFromStr(hex.EncodeToString(in.OutpointHash))
		if err != nil {
			return sigs, err
		}
		outpoint := wire.NewOutPoint(ch, in.OutpointIndex)
		input := wire.NewTxIn(outpoint, []byte{}, [][]byte{})
		tx.TxIn = append(tx.TxIn, input)
	}
	for _, out := range outs {
		scriptPubKey, err := txscript.PayToAddrScript(out.Address)
		if err != nil {
			return nil, err
		}
		output := wire.NewTxOut(out.Value, scriptPubKey)
		tx.TxOut = append(tx.TxOut, output)
	}

	// Subtract fee
	txType := spvwallet.P2SH_2of3_Multisig
	_, err := spvwallet.LockTimeFromRedeemScript(redeemScript)
	if err == nil {
		txType = spvwallet.P2SH_Multisig_Timelock_2Sigs
	}
	estimatedSize := spvwallet.EstimateSerializeSize(len(ins), tx.TxOut, false, txType)
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

	hashes := txscript.NewTxSigHashes(tx)
	for i := range tx.TxIn {
		sig, err := txscript.RawTxInWitnessSignature(tx, hashes, i, ins[i].Value, redeemScript, txscript.SigHashAll, signingKey)
		if err != nil {
			continue
		}
		bs := wallet.Signature{InputIndex: uint32(i), Signature: sig}
		sigs = append(sigs, bs)
	}
	return sigs, nil
}

func (w *BitcoindWallet) Multisign(ins []wallet.TransactionInput, outs []wallet.TransactionOutput, sigs1 []wallet.Signature, sigs2 []wallet.Signature, redeemScript []byte, feePerByte uint64, broadcast bool) ([]byte, error) {
	<-w.initChan
	tx := wire.NewMsgTx(1)
	for _, in := range ins {
		ch, err := chainhash.NewHashFromStr(hex.EncodeToString(in.OutpointHash))
		if err != nil {
			return nil, err
		}
		outpoint := wire.NewOutPoint(ch, in.OutpointIndex)
		input := wire.NewTxIn(outpoint, []byte{}, [][]byte{})
		tx.TxIn = append(tx.TxIn, input)
	}
	for _, out := range outs {
		scriptPubKey, err := txscript.PayToAddrScript(out.Address)
		if err != nil {
			return nil, err
		}
		output := wire.NewTxOut(out.Value, scriptPubKey)
		tx.TxOut = append(tx.TxOut, output)
	}

	// Subtract fee
	txType := spvwallet.P2SH_2of3_Multisig
	_, err := spvwallet.LockTimeFromRedeemScript(redeemScript)
	if err == nil {
		txType = spvwallet.P2SH_Multisig_Timelock_2Sigs
	}
	estimatedSize := spvwallet.EstimateSerializeSize(len(ins), tx.TxOut, false, txType)
	fee := estimatedSize * int(feePerByte)
	if len(tx.TxOut) > 0 {
		feePerOutput := fee / len(tx.TxOut)
		for _, output := range tx.TxOut {
			output.Value -= int64(feePerOutput)
		}
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	// Check if time locked
	var timeLocked bool
	if redeemScript[0] == txscript.OP_IF {
		timeLocked = true
	}

	for i, input := range tx.TxIn {
		var sig1 []byte
		var sig2 []byte
		for _, sig := range sigs1 {
			if int(sig.InputIndex) == i {
				sig1 = sig.Signature
				break
			}
		}
		for _, sig := range sigs2 {
			if int(sig.InputIndex) == i {
				sig2 = sig.Signature
				break
			}
		}

		witness := wire.TxWitness{[]byte{}, sig1, sig2}

		if timeLocked {
			witness = append(witness, []byte{0x01})
		}
		witness = append(witness, redeemScript)
		input.Witness = witness
	}
	// broadcast
	if broadcast {
		_, err = w.rpcClient.SendRawTransaction(tx, false)
		if err != nil {
			return nil, err
		}
	}
	var buf bytes.Buffer
	tx.BtcEncode(&buf, wire.ProtocolVersion, wire.WitnessEncoding)
	return buf.Bytes(), nil
}

func (w *BitcoindWallet) SweepAddress(ins []wallet.TransactionInput, address *btc.Address, key *hd.ExtendedKey, redeemScript *[]byte, feeLevel wallet.FeeLevel) (*chainhash.Hash, error) {
	<-w.initChan
	var internalAddr btc.Address
	if address != nil {
		internalAddr = *address
	} else {
		internalAddr = w.CurrentAddress(wallet.INTERNAL)
	}
	script, err := txscript.PayToAddrScript(internalAddr)
	if err != nil {
		return nil, err
	}

	var val int64
	var inputs []*wire.TxIn
	additionalPrevScripts := make(map[wire.OutPoint][]byte)
	for _, in := range ins {
		val += in.Value
		ch, err := chainhash.NewHashFromStr(hex.EncodeToString(in.OutpointHash))
		if err != nil {
			return nil, err
		}
		script, err := txscript.PayToAddrScript(in.LinkedAddress)
		if err != nil {
			return nil, err
		}
		outpoint := wire.NewOutPoint(ch, in.OutpointIndex)
		input := wire.NewTxIn(outpoint, []byte{}, [][]byte{})
		inputs = append(inputs, input)
		additionalPrevScripts[*outpoint] = script
	}
	out := wire.NewTxOut(val, script)

	txType := spvwallet.P2PKH
	if redeemScript != nil {
		txType = spvwallet.P2SH_1of2_Multisig
		_, err := spvwallet.LockTimeFromRedeemScript(*redeemScript)
		if err == nil {
			txType = spvwallet.P2SH_Multisig_Timelock_1Sig
		}
	}
	estimatedSize := spvwallet.EstimateSerializeSize(len(ins), []*wire.TxOut{out}, false, txType)

	// Calculate the fee
	feePerByte := int(w.GetFeePerByte(feeLevel))
	fee := estimatedSize * feePerByte

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
	privKey, err := key.ECPrivKey()
	if err != nil {
		return nil, err
	}
	pk := privKey.PubKey().SerializeCompressed()
	addressPub, err := btc.NewAddressPubKey(pk, w.params)

	getKey := txscript.KeyClosure(func(addr btc.Address) (*btcec.PrivateKey, bool, error) {
		if addressPub.EncodeAddress() == addr.EncodeAddress() {
			wif, err := btc.NewWIF(privKey, w.params, true)
			if err != nil {
				return nil, false, err
			}
			return wif.PrivKey, wif.CompressPubKey, nil
		}
		return nil, false, errors.New("Not found")
	})
	getScript := txscript.ScriptClosure(func(addr btc.Address) ([]byte, error) {
		if redeemScript == nil {
			return []byte{}, nil
		}
		return *redeemScript, nil
	})

	// Check if time locked
	var timeLocked bool
	if redeemScript != nil {
		rs := *redeemScript
		if rs[0] == txscript.OP_IF {
			timeLocked = true
			tx.Version = 2
		}
		for _, txIn := range tx.TxIn {
			locktime, err := spvwallet.LockTimeFromRedeemScript(*redeemScript)
			if err != nil {
				return nil, err
			}
			txIn.Sequence = locktime
		}
	}

	hashes := txscript.NewTxSigHashes(tx)
	for i, txIn := range tx.TxIn {
		if redeemScript == nil {
			prevOutScript := additionalPrevScripts[txIn.PreviousOutPoint]
			script, err := txscript.SignTxOutput(w.params,
				tx, i, prevOutScript, txscript.SigHashAll, getKey,
				getScript, txIn.SignatureScript)
			if err != nil {
				return nil, errors.New("Failed to sign transaction")
			}
			txIn.SignatureScript = script
		} else {
			sig, err := txscript.RawTxInWitnessSignature(tx, hashes, i, ins[i].Value, *redeemScript, txscript.SigHashAll, privKey)
			if err != nil {
				return nil, err
			}
			var witness wire.TxWitness
			if timeLocked {
				witness = wire.TxWitness{sig, []byte{}}
			} else {
				witness = wire.TxWitness{[]byte{}, sig}
			}
			witness = append(witness, *redeemScript)
			txIn.Witness = witness
		}
	}

	// broadcast
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

func (w *BitcoindWallet) AddTransactionListener(callback func(wallet.TransactionCallback)) {
	w.listeners = append(w.listeners, callback)
}

func (w *BitcoindWallet) GenerateMultisigScript(keys []hd.ExtendedKey, threshold int, timeout time.Duration, timeoutKey *hd.ExtendedKey) (addr btc.Address, redeemScript []byte, err error) {
	if uint32(timeout.Hours()) > 0 && timeoutKey == nil {
		return nil, nil, errors.New("Timeout key must be non nil when using an escrow timeout")
	}

	if len(keys) < threshold {
		return nil, nil, fmt.Errorf("unable to generate multisig script with "+
			"%d required signatures when there are only %d public "+
			"keys available", threshold, len(keys))
	}

	var ecKeys []*btcec.PublicKey
	for _, key := range keys {
		ecKey, err := key.ECPubKey()
		if err != nil {
			return nil, nil, err
		}
		ecKeys = append(ecKeys, ecKey)
	}

	builder := txscript.NewScriptBuilder()
	if uint32(timeout.Hours()) == 0 {

		builder.AddInt64(int64(threshold))
		for _, key := range ecKeys {
			builder.AddData(key.SerializeCompressed())
		}
		builder.AddInt64(int64(len(ecKeys)))
		builder.AddOp(txscript.OP_CHECKMULTISIG)

	} else {
		ecKey, err := timeoutKey.ECPubKey()
		if err != nil {
			return nil, nil, err
		}
		sequenceLock := blockchain.LockTimeToSequence(false, uint32(timeout.Hours()*6))
		builder.AddOp(txscript.OP_IF)
		builder.AddInt64(int64(threshold))
		for _, key := range ecKeys {
			builder.AddData(key.SerializeCompressed())
		}
		builder.AddInt64(int64(len(ecKeys)))
		builder.AddOp(txscript.OP_CHECKMULTISIG)
		builder.AddOp(txscript.OP_ELSE).
			AddInt64(int64(sequenceLock)).
			AddOp(txscript.OP_CHECKSEQUENCEVERIFY).
			AddOp(txscript.OP_DROP).
			AddData(ecKey.SerializeCompressed()).
			AddOp(txscript.OP_CHECKSIG).
			AddOp(txscript.OP_ENDIF)
	}
	redeemScript, err = builder.Script()
	if err != nil {
		return nil, nil, err
	}

	witnessProgram := sha256.Sum256(redeemScript)

	addr, err = btc.NewAddressWitnessScriptHash(witnessProgram[:], w.params)
	if err != nil {
		return nil, nil, err
	}
	return addr, redeemScript, nil
}

func (w *BitcoindWallet) AddWatchedAddress(addr btc.Address) error {
	select {
	case <-w.initChan:
		return w.addWatchedScript(addr)
	default:
		w.addrsToWatch = append(w.addrsToWatch, addr)
	}
	return nil
}

func (w *BitcoindWallet) addWatchedScript(addr btc.Address) error {
	return w.rpcClient.ImportAddressRescan(addr.EncodeAddress(), false)
}

func (w *BitcoindWallet) ReSyncBlockchain(fromDate time.Time) {
	<-w.initChan
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

func DefaultSocksPort(controlPort int) int {
	socksPort := 9050
	if controlPort == 9151 || controlPort == 9051 {
		controlPort--
		socksPort = controlPort
	}
	return socksPort
}
