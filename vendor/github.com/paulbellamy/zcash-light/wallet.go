package zcash

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/OpenBazaar/multiwallet/client"
	"github.com/OpenBazaar/multiwallet/keys"
	"github.com/OpenBazaar/spvwallet"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	btc "github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/coinset"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/btcsuite/btcwallet/wallet/txrules"
	"github.com/op/go-logging"
	b39 "github.com/tyler-smith/go-bip39"
	"golang.org/x/net/proxy"
)

const (
	OverwinterProtocolVersion = 170005
)

var log = logging.MustGetLogger("zcash")

type Wallet struct {
	Config
	keyManager             *keys.KeyManager
	masterPrivateKey       *hd.ExtendedKey
	masterPublicKey        *hd.ExtendedKey
	insight                InsightClient
	isOverwinter           bool
	txStore                *TxStore
	initChan               chan struct{}
	addrSubscriptions      map[btc.Address]struct{}
	addrSubscriptionsMutex sync.Mutex
}

type Config struct {
	Mnemonic string
	Params   *chaincfg.Params
	DB       wallet.Datastore
	Proxy    proxy.Dialer
}

// Stubbable for testing
var (
	newInsightClient = func(url string, proxyDialer proxy.Dialer) (InsightClient, error) {
		return client.NewInsightClient(url, proxyDialer)
	}

	// TODO: Find if there are insight apis for networks other than mainnet, we
	// really shouldn't be using mainnet insight apis for test and regression
	// networks.
	insightURLs = map[string]string{
		chaincfg.TestNet3Params.Name: "https://explorer.testnet.z.cash/api/",
		chaincfg.MainNetParams.Name:  "https://zcashnetwork.info/api/",
	}
)

type InsightClient interface {
	GetInfo() (*client.Info, error)
	GetBestBlock() (*client.Block, error)
	GetBlocksBefore(time.Time, int) (*client.BlockList, error)
	GetTransactions(addrs []btc.Address) ([]client.Transaction, error)
	GetRawTransaction(txid string) ([]byte, error)
	BlockNotify() <-chan client.Block
	TransactionNotify() <-chan client.Transaction
	Broadcast(tx []byte) (string, error)
	EstimateFee(nbBlocks int) (int, error)
	ListenAddress(addr btc.Address)
	Close()
}

func NewWallet(config Config) (*Wallet, error) {
	seed := b39.NewSeed(config.Mnemonic, "")
	mPrivKey, _ := hd.NewMaster(seed, config.Params)
	mPubKey, _ := mPrivKey.Neuter()
	keyManager, err := keys.NewKeyManager(config.DB.Keys(), config.Params, mPrivKey, wallet.Zcash, KeyToAddress)
	if err != nil {
		return nil, fmt.Errorf("error initializing key manager: %v", err)
	}
	insightURL, ok := insightURLs[config.Params.Name]
	if !ok {
		return nil, fmt.Errorf("unsupported network: %v", config.Params.Name)
	}
	insight, err := newInsightClient(insightURL, config.Proxy)
	if err != nil {
		return nil, fmt.Errorf("error initializing insight client: %v", err)
	}
	txStore, err := NewTxStore(config.Params, config.DB, keyManager)
	if err != nil {
		return nil, fmt.Errorf("error initializing txstore: %v", err)
	}

	w := &Wallet{
		Config:            config,
		keyManager:        keyManager,
		masterPrivateKey:  mPrivKey,
		masterPublicKey:   mPubKey,
		insight:           insight,
		txStore:           txStore,
		initChan:          make(chan struct{}),
		addrSubscriptions: make(map[btc.Address]struct{}),
	}

	info, err := w.insight.GetInfo()
	if err != nil {
		return nil, fmt.Errorf("error loading insight api status: %v", err)
	}
	w.isOverwinter = info.ProtocolVersion >= OverwinterProtocolVersion

	return w, nil
}

// TestNetworkEnabled indicates if the current network being used is Test Network
func (w *Wallet) TestNetworkEnabled() bool {
	return w.Params().Name == chaincfg.TestNet3Params.Name
}

// RegressionNetworkEnabled indicates if the current network being used is Regression Network
func (w *Wallet) RegressionNetworkEnabled() bool {
	return false
}

// MainNetworkEnabled indicates if the current network being used is the live Network
func (w *Wallet) MainNetworkEnabled() bool {
	return w.Params().Name == chaincfg.MainNetParams.Name
}

func (w *Wallet) Start() {
	go func() {
		w.subscribeToAllAddresses()
		w.loadInitialTransactions()
		go w.watchTransactions()
		close(w.initChan)
	}()
}

func (w *Wallet) onTxn(txn client.Transaction) error {
	raw, err := w.insight.GetRawTransaction(txn.Txid)
	if err != nil {
		return err
	}
	var tx Transaction
	if err := tx.UnmarshalBinary(raw); err != nil {
		return err
	}
	_, err = w.txStore.Ingest(&tx, raw, int32(txn.BlockHeight))
	return err
}

func (w *Wallet) subscribeToAllAddresses() {
	keys := w.keyManager.GetKeys()
	for _, k := range keys {
		if addr, err := KeyToAddress(k, w.Params()); err == nil {
			w.addWatchedAddr(addr)
		}
	}
	scripts, _ := w.DB.WatchedScripts().GetAll()
	for _, script := range scripts {
		if addr, err := w.ScriptToAddress(script); err == nil {
			w.addWatchedAddr(addr)
		}
	}
}

func (w *Wallet) loadInitialTransactions() {
	txns, err := w.insight.GetTransactions(w.allWatchedAddrs())
	if err != nil {
		log.Error(err)
		return
	}
	for _, txn := range txns {
		if err := w.onTxn(txn); err != nil {
			log.Error(err)
			return
		}
	}
}

func (w *Wallet) watchTransactions() {
	for {
		select {
		case block, ok := <-w.insight.BlockNotify():
			if !ok {
				return
			}
			// Insight only notifies us of a transaction when it is first seen (i.e.
			// not mined yet), so we need to watch for new blocks, and check when
			// txns are mined into them.
			for _, txid := range block.Tx {
				if err := w.onTxn(client.Transaction{Txid: txid, BlockHeight: block.Height}); err != nil {
					log.Errorf("error fetching transaction %v: %v", txid, err)
				}
			}
		case txn, ok := <-w.insight.TransactionNotify():
			if !ok {
				return
			}
			if err := w.onTxn(txn); err != nil {
				log.Errorf("error fetching transaction %v: %v", txn.Txid, err)
			} else {
				log.Debugf("fetched transaction %v", txn.Txid)
			}
		}
	}
}

// Return the network parameters
func (w *Wallet) Params() *chaincfg.Params {
	return w.Config.Params
}

// Returns the type of crytocurrency this wallet implements
func (w *Wallet) CurrencyCode() string {
	if w.MainNetworkEnabled() {
		return "zec"
	}
	return "tzec"
}

// Check if this amount is considered dust
func (w *Wallet) IsDust(amount int64) bool {
	return txrules.IsDustAmount(btc.Amount(amount), 25, txrules.DefaultRelayFeePerKb)
}

// Get the master private key
func (w *Wallet) MasterPrivateKey() *hd.ExtendedKey {
	return w.masterPrivateKey
}

// Get the master public key
func (w *Wallet) MasterPublicKey() *hd.ExtendedKey {
	return w.masterPublicKey
}

// Get the current address for the given purpose
// TODO: Handle these errors
func (w *Wallet) CurrentAddress(purpose wallet.KeyPurpose) btc.Address {
	key, _ := w.keyManager.GetCurrentKey(purpose)
	addr, _ := KeyToAddress(key, w.Params())
	return addr
}

// Returns a fresh address that has never been returned by this function
func (w *Wallet) NewAddress(purpose wallet.KeyPurpose) btc.Address {
	key, _ := w.keyManager.GetFreshKey(purpose)
	addr, _ := KeyToAddress(key, w.Params())
	w.DB.Keys().MarkKeyAsUsed(addr.ScriptAddress())
	w.addWatchedAddr(addr)
	w.txStore.PopulateAdrs()
	return addr
}

// Parse the address string and return an address interface
func (w *Wallet) DecodeAddress(addr string) (btc.Address, error) {
	return DecodeAddress(addr, w.Params())
}

// Turn the given output script into an address
func (w *Wallet) ScriptToAddress(script []byte) (btc.Address, error) {
	return ExtractPkScriptAddrs(script, w.Params())
}

// Turn the given address into an output script
func (w *Wallet) AddressToScript(addr btc.Address) ([]byte, error) {
	return PayToAddrScript(addr)
}

// Returns if the wallet has the key for the given address
func (w *Wallet) HasKey(addr btc.Address) bool {
	<-w.initChan
	_, err := w.keyManager.GetKeyForScript(addr.ScriptAddress())
	return err == nil
}

// Get the confirmed and unconfirmed balances
// TODO: Handle this error
// TODO: Maybe we could just use insight api for this
func (w *Wallet) Balance() (confirmed, unconfirmed int64) {
	<-w.initChan
	utxos, _ := w.DB.Utxos().GetAll()
	stxos, _ := w.DB.Stxos().GetAll()
	for _, utxo := range utxos {
		if !utxo.WatchOnly {
			if utxo.AtHeight > 0 {
				confirmed += utxo.Value
			} else {
				if w.checkIfStxoIsConfirmed(utxo, stxos) {
					confirmed += utxo.Value
				} else {
					unconfirmed += utxo.Value
				}
			}
		}
	}
	return confirmed, unconfirmed
}

func (w *Wallet) checkIfStxoIsConfirmed(utxo wallet.Utxo, stxos []wallet.Stxo) bool {
	for _, stxo := range stxos {
		if !stxo.Utxo.WatchOnly {
			if stxo.SpendTxid.IsEqual(&utxo.Op.Hash) {
				if stxo.SpendHeight > 0 {
					return true
				} else {
					return w.checkIfStxoIsConfirmed(stxo.Utxo, stxos)
				}
			} else if stxo.Utxo.IsEqual(&utxo) {
				if stxo.Utxo.AtHeight > 0 {
					return true
				} else {
					return false
				}
			}
		}
	}
	return false
}

func (w *Wallet) Transactions() ([]wallet.Txn, error) {
	<-w.initChan
	return w.DB.Txns().GetAll(false)
}

// Get info on a specific transaction
func (w *Wallet) GetTransaction(txid chainhash.Hash) (wallet.Txn, error) {
	<-w.initChan
	return w.DB.Txns().Get(txid)
}

// Get the height and best hash of the blockchain
// TODO: We should fetch all blocks and watch for changes here instead of being
// so dependent on the insight api
func (w *Wallet) ChainTip() (uint32, chainhash.Hash) {
	<-w.initChan
	block, err := w.insight.GetBestBlock()
	if err != nil {
		log.Errorf("error fetching latest block: %v", err)
		return 0, chainhash.Hash{}
	}
	hash, _ := chainhash.NewHashFromStr(block.Hash)
	return uint32(block.Height), *hash
}

func (w *Wallet) Spend(amount int64, addr btc.Address, feeLevel wallet.FeeLevel) (*chainhash.Hash, error) {
	<-w.initChan
	txn, err := w.buildTxn(amount, addr, feeLevel)
	if err != nil {
		return nil, err
	}
	return w.broadcastTx(txn)
}

func (w *Wallet) broadcastTx(tx *Transaction) (*chainhash.Hash, error) {
	b, err := tx.MarshalBinary()
	if err != nil {
		return nil, err
	}

	hash, err := w.insight.Broadcast(b)
	if err != nil {
		return nil, err
	}

	// Our own tx; don't keep track of false positives
	if _, err := w.txStore.Ingest(tx, b, 0); err != nil {
		return nil, err
	}

	return chainhash.NewHashFromStr(hash)
}

func (w *Wallet) buildTxn(amount int64, addr btc.Address, feeLevel wallet.FeeLevel) (*Transaction, error) {
	script, err := w.AddressToScript(addr)
	if err != nil {
		return nil, err
	}
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
	inputSource := func(target btc.Amount) (total btc.Amount, inputs []Input, scripts [][]byte, err error) {
		coinSelector := coinset.MaxValueAgeCoinSelector{MaxInputs: 10000, MinChangeAmount: btc.Amount(0)}
		coins, err := coinSelector.CoinSelect(target, coins)
		if err != nil {
			return total, inputs, scripts, wallet.ErrorInsuffientFunds
		}
		additionalPrevScripts = make(map[wire.OutPoint][]byte)
		additionalKeysByAddress = make(map[string]*btc.WIF)
		for _, c := range coins.Coins() {
			total += c.Value()
			outpoint := wire.NewOutPoint(c.Hash(), c.Index())
			inputs = append(inputs, Input{
				PreviousOutPoint: *outpoint,
				Sequence:         0, // Opt-in RBF so we can bump fees
			})
			additionalPrevScripts[*outpoint] = c.PkScript()
			key := coinMap[c]
			addr, err := KeyToAddress(key, w.Params())
			if err != nil {
				continue
			}
			privKey, err := key.ECPrivKey()
			if err != nil {
				continue
			}
			wif, _ := btc.NewWIF(privKey, w.Params(), true)
			additionalKeysByAddress[addr.EncodeAddress()] = wif
		}
		return total, inputs, scripts, nil
	}

	// Get the fee per kilobyte
	feePerKB := int64(w.GetFeePerByte(feeLevel)) * 1000

	// outputs
	outputs := []Output{{Value: amount, ScriptPubKey: script}}

	// Create change source
	changeSource := func() ([]byte, error) {
		addr := w.CurrentAddress(wallet.INTERNAL)
		script, err := w.AddressToScript(addr)
		if err != nil {
			return []byte{}, err
		}
		return script, nil
	}

	authoredTx, err := NewUnsignedTransaction(outputs, btc.Amount(feePerKB), inputSource, changeSource, w.isOverwinter)
	if err != nil {
		return nil, err
	}

	// BIP 69 sorting
	authoredTx.Sort()

	// Sign tx
	getKey := txscript.KeyClosure(func(addr btc.Address) (*btcec.PrivateKey, bool, error) {
		wif := additionalKeysByAddress[addr.EncodeAddress()]
		return wif.PrivKey, wif.CompressPubKey, nil
	})
	getScript := txscript.ScriptClosure(func(
		addr btc.Address) ([]byte, error) {
		return []byte{}, nil
	})
	for i, input := range authoredTx.Inputs {
		prevOutScript := additionalPrevScripts[input.PreviousOutPoint]
		signature, err := ProduceSignature(w.Params(), authoredTx, i, prevOutScript, txscript.SigHashAll, getKey, getScript, input.SignatureScript)
		if err != nil {
			return nil, fmt.Errorf("failed to sign transaction: %v", err)
		}
		authoredTx.Inputs[i].SignatureScript = signature
	}
	return authoredTx, nil
}

func (w *Wallet) gatherCoins() (map[coinset.Coin]*hd.ExtendedKey, error) {
	<-w.initChan
	tipHeight, _ := w.ChainTip()
	m := make(map[coinset.Coin]*hd.ExtendedKey)
	utxos, err := w.DB.Utxos().GetAll()
	if err != nil {
		return m, err
	}
	for _, u := range utxos {
		if u.WatchOnly || u.AtHeight <= 0 || u.AtHeight >= int32(tipHeight) {
			// not yet mined, or zero confirmations, so not spendable
			continue
		}

		c := spvwallet.NewCoin(u.Op.Hash.CloneBytes(), u.Op.Index, btc.Amount(u.Value), int64(tipHeight)-int64(u.AtHeight), u.ScriptPubkey)

		addr, err := w.ScriptToAddress(u.ScriptPubkey)
		if err != nil {
			return nil, err
		}
		hdKey, err := w.keyManager.GetKeyForScript(addr.ScriptAddress())
		if err != nil {
			return nil, err
		}
		m[c] = hdKey
	}
	return m, nil
}

func (w *Wallet) BumpFee(txid chainhash.Hash) (*chainhash.Hash, error) {
	<-w.initChan
	tipHeight, _ := w.ChainTip()
	tx, err := w.DB.Txns().Get(txid)
	if err != nil {
		return nil, err
	}
	if tx.WatchOnly {
		return nil, fmt.Errorf("not found")
	}
	if tx.Height <= 0 || tx.Height > int32(tipHeight) {
		return nil, spvwallet.BumpFeeAlreadyConfirmedError
	}
	unspent, err := w.DB.Utxos().GetAll()
	if err != nil {
		return nil, err
	}
	for _, u := range unspent {
		if u.Op.Hash.String() == txid.String() {
			if u.AtHeight > 0 && u.AtHeight < int32(tipHeight) {
				return nil, spvwallet.BumpFeeAlreadyConfirmedError
			}
			addr, err := w.ScriptToAddress(u.ScriptPubkey)
			if err != nil {
				continue
			}
			hdKey, err := w.keyManager.GetKeyForScript(addr.ScriptAddress())
			if err != nil {
				continue
			}
			transactionID, err := w.SweepAddress([]wallet.Utxo{u}, nil, hdKey, nil, wallet.FEE_BUMP)
			if err != nil {
				return nil, err
			}
			return transactionID, nil

		}
	}
	return nil, spvwallet.BumpFeeNotFoundError
}

// Get the current fee per byte
func (w *Wallet) GetFeePerByte(feeLevel wallet.FeeLevel) uint64 {
	<-w.initChan
	defaultFee := uint64(50)
	var nBlocks int
	switch feeLevel {
	case wallet.PRIOIRTY:
		nBlocks = 1
	case wallet.NORMAL:
		nBlocks = 3
	case wallet.ECONOMIC:
		nBlocks = 6
	default:
		return defaultFee
	}
	feePerKb, err := w.insight.EstimateFee(nBlocks)
	if err != nil {
		return defaultFee
	}
	if feePerKb <= 0 {
		return defaultFee
	}
	fee := feePerKb / 1000
	return uint64(fee)
}

// Calculates the estimated size of the transaction and returns the total fee for the given feePerByte
func (w *Wallet) EstimateFee(ins []wallet.TransactionInput, outs []wallet.TransactionOutput, feePerByte uint64) uint64 {
	tx := wire.NewMsgTx(wire.TxVersion)
	for _, out := range outs {
		output := wire.NewTxOut(out.Value, out.ScriptPubKey)
		tx.TxOut = append(tx.TxOut, output)
	}
	estimatedSize := spvwallet.EstimateSerializeSize(len(ins), tx.TxOut, false, spvwallet.P2PKH)
	fee := estimatedSize * int(feePerByte)
	return uint64(fee)
}

// Build a spend transaction for the amount and return the transaction fee
func (w *Wallet) EstimateSpendFee(amount int64, feeLevel wallet.FeeLevel) (uint64, error) {
	<-w.initChan
	addr, err := w.DecodeAddress("t1VpYecBW4UudbGcy4ufh61eWxQCoFaUrPs")
	if err != nil {
		return 0, err
	}
	txn, err := w.buildTxn(amount, addr, feeLevel)
	if err != nil {
		return 0, err
	}
	var outval int64
	for _, output := range txn.Outputs {
		outval += output.Value
	}
	var inval int64
	utxos, err := w.DB.Utxos().GetAll()
	if err != nil {
		return 0, err
	}
	for _, input := range txn.Inputs {
		for _, utxo := range utxos {
			if outpointsEqual(utxo.Op, input.PreviousOutPoint) {
				inval += int64(utxo.Value)
				break
			}
		}
	}
	if inval < outval {
		return 0, errors.New("Error building transaction: inputs less than outputs")
	}
	return uint64(inval - outval), err
}

func outpointsEqual(a, b wire.OutPoint) bool {
	return a.Hash.String() == b.Hash.String() && a.Index == b.Index
}

// Build and broadcast a transaction that sweeps all coins from an address. If it is a p2sh multisig, the redeemScript must be included
func (w *Wallet) SweepAddress(utxos []wallet.Utxo, address *btc.Address, key *hd.ExtendedKey, redeemScript *[]byte, feeLevel wallet.FeeLevel) (*chainhash.Hash, error) {
	<-w.initChan
	var internalAddr btc.Address
	if address != nil {
		internalAddr = *address
	} else {
		internalAddr = w.CurrentAddress(wallet.INTERNAL)
	}
	script, err := w.AddressToScript(internalAddr)
	if err != nil {
		return nil, err
	}

	var val int64
	var inputs []Input
	additionalPrevScripts := make(map[wire.OutPoint][]byte)
	for _, u := range utxos {
		val += u.Value
		inputs = append(inputs, Input{PreviousOutPoint: u.Op})
		additionalPrevScripts[u.Op] = u.ScriptPubkey
	}
	out := Output{Value: val, ScriptPubKey: script}

	txType := spvwallet.P2PKH
	if redeemScript != nil {
		txType = spvwallet.P2SH_1of2_Multisig
	}

	estimatedSize := EstimateSerializeSize(len(utxos), []Output{out}, false, txType)

	// Calculate the fee
	feePerKb, err := w.insight.EstimateFee(1)
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

	tx := &Transaction{
		Version: 1,
		Inputs:  inputs,
		Outputs: []Output{out},
	}

	// BIP 69 sorting
	tx.Sort()

	// Sign tx
	getKey := txscript.KeyClosure(func(addr btc.Address) (*btcec.PrivateKey, bool, error) {
		privKey, err := key.ECPrivKey()
		if err != nil {
			return nil, false, err
		}
		wif, err := btc.NewWIF(privKey, w.Params(), true)
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

	for i, input := range tx.Inputs {
		prevOutScript := additionalPrevScripts[input.PreviousOutPoint]
		signature, err := ProduceSignature(w.Params(), tx, i, prevOutScript, txscript.SigHashAll, getKey, getScript, input.SignatureScript)
		if err != nil {
			return nil, fmt.Errorf("failed to sign transaction: %v", err)
		}
		input.SignatureScript = signature
	}

	// Broadcast
	if _, err = w.broadcastTx(tx); err != nil {
		return nil, err
	}
	txid := tx.TxHash()
	return &txid, nil
}

// Create a signature for a multisig transaction
func (w *Wallet) CreateMultisigSignature(ins []wallet.TransactionInput, outs []wallet.TransactionOutput, key *hd.ExtendedKey, redeemScript []byte, feePerByte uint64) ([]wallet.Signature, error) {
	if len(outs) <= 0 {
		return nil, fmt.Errorf("transaction has no outputs")
	}
	var sigs []wallet.Signature
	tx := &Transaction{Version: 1}
	for _, in := range ins {
		ch, err := chainhash.NewHashFromStr(hex.EncodeToString(in.OutpointHash))
		if err != nil {
			return sigs, err
		}
		outpoint := wire.NewOutPoint(ch, in.OutpointIndex)
		tx.Inputs = append(tx.Inputs, Input{PreviousOutPoint: *outpoint})
	}
	for _, out := range outs {
		tx.Outputs = append(tx.Outputs, Output{Value: out.Value, ScriptPubKey: out.ScriptPubKey})
	}

	// Subtract fee
	estimatedSize := EstimateSerializeSize(len(ins), tx.Outputs, false, spvwallet.P2SH_2of3_Multisig)
	fee := estimatedSize * int(feePerByte)
	feePerOutput := fee / len(tx.Outputs)
	for _, output := range tx.Outputs {
		output.Value -= int64(feePerOutput)
	}

	// BIP 69 sorting
	tx.Sort()

	signingKey, err := key.ECPrivKey()
	if err != nil {
		return sigs, err
	}

	addr, err := KeyToAddress(key, w.Params())
	if err != nil {
		return sigs, err
	}

	getKey := txscript.KeyClosure(func(addr btc.Address) (*btcec.PrivateKey, bool, error) {
		return signingKey, false, nil
	})
	getScript := txscript.ScriptClosure(func(addr btc.Address) ([]byte, error) {
		return redeemScript, nil
	})

	for i := range tx.Inputs {
		// TODO: Check Sign1 is the right thing to use here, it seems like we could refactor this to ProduceSignature, maybe.
		creator := TransactionSignatureCreator(getKey, getScript, tx, i, txscript.SigHashAll)
		sig, ok := Sign1(addr, creator, redeemScript, tx.VersionGroupID)
		if !ok {
			continue
		}
		bs := wallet.Signature{InputIndex: uint32(i), Signature: sig}
		sigs = append(sigs, bs)
	}
	return sigs, nil
}

// Combine signatures and optionally broadcast
func (w *Wallet) Multisign(ins []wallet.TransactionInput, outs []wallet.TransactionOutput, sigs1 []wallet.Signature, sigs2 []wallet.Signature, redeemScript []byte, feePerByte uint64, broadcast bool) ([]byte, error) {
	<-w.initChan
	tx := &Transaction{Version: 1}
	for _, in := range ins {
		ch, err := chainhash.NewHashFromStr(hex.EncodeToString(in.OutpointHash))
		if err != nil {
			return nil, err
		}
		outpoint := wire.NewOutPoint(ch, in.OutpointIndex)
		tx.Inputs = append(tx.Inputs, Input{PreviousOutPoint: *outpoint})
	}
	for _, out := range outs {
		tx.Outputs = append(tx.Outputs, Output{Value: out.Value, ScriptPubKey: out.ScriptPubKey})
	}

	// Subtract fee
	estimatedSize := EstimateSerializeSize(len(ins), tx.Outputs, false, spvwallet.P2SH_2of3_Multisig)
	fee := estimatedSize * int(feePerByte)
	feePerOutput := fee / len(tx.Outputs)
	for _, output := range tx.Outputs {
		output.Value -= int64(feePerOutput)
	}

	// BIP 69 sorting
	tx.Sort()

	for i, input := range tx.Inputs {
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
	if broadcast {
		if _, err := w.broadcastTx(tx); err != nil {
			return nil, err
		}
	}
	return tx.MarshalBinary()
}

// Generate a multisig script from public keys. If a timeout is included the returned script should be a timelocked escrow which releases using the timeoutKey.
func (w *Wallet) GenerateMultisigScript(keys []hd.ExtendedKey, threshold int, timeout time.Duration, timeoutKey *hd.ExtendedKey) (addr btc.Address, redeemScript []byte, err error) {
	var addrPubKeys []*btc.AddressPubKey
	for _, key := range keys {
		ecKey, err := key.ECPubKey()
		if err != nil {
			return nil, nil, err
		}
		k, err := btc.NewAddressPubKey(ecKey.SerializeCompressed(), w.Params())
		if err != nil {
			return nil, nil, err
		}
		addrPubKeys = append(addrPubKeys, k)
	}
	redeemScript, err = txscript.MultiSigScript(addrPubKeys, threshold)
	if err != nil {
		return nil, nil, err
	}
	addr, err = NewAddressScriptHash(redeemScript, w.Params())
	if err != nil {
		return nil, nil, err
	}
	return addr, redeemScript, nil
}

// Add a script to the wallet and get notifications back when coins are received or spent from it
func (w *Wallet) AddWatchedScript(script []byte) error {
	if addr, err := w.ScriptToAddress(script); err == nil {
		w.addWatchedAddr(addr)
	}
	err := w.DB.WatchedScripts().Put(script)
	w.txStore.PopulateAdrs()
	return err
}

func (w *Wallet) addWatchedAddr(addr btc.Address) {
	w.addrSubscriptionsMutex.Lock()
	if _, ok := w.addrSubscriptions[addr]; !ok {
		w.addrSubscriptions[addr] = struct{}{}
		w.insight.ListenAddress(addr)
	}
	w.addrSubscriptionsMutex.Unlock()
}

func (w *Wallet) allWatchedAddrs() []btc.Address {
	w.addrSubscriptionsMutex.Lock()
	var addrs []btc.Address
	for addr := range w.addrSubscriptions {
		addrs = append(addrs, addr)
	}
	w.addrSubscriptionsMutex.Unlock()
	return addrs
}

// Add a callback for incoming transactions
func (w *Wallet) AddTransactionListener(callback func(wallet.TransactionCallback)) {
	w.txStore.listeners = append(w.txStore.listeners, callback)
}

func (w *Wallet) ReSyncBlockchain(fromDate time.Time) {
	<-w.initChan
	// find last block before time
	height, err := w.findHeightBeforeTime(fromDate)
	if err != nil {
		log.Errorf("unable to resync blockchain: %v", err)
		return
	}

	// delete transactions after that block
	txns, err := w.DB.Txns().GetAll(true)
	if err != nil {
		log.Errorf("unable to resync blockchain: %v", err)
		return
	}
	for _, t := range txns {
		if t.Height < height {
			continue
		}
		hash, _ := chainhash.NewHashFromStr(t.Txid)
		if err := w.DB.Txns().Delete(hash); err != nil {
			log.Errorf("error resyncing blockchain: %v", err)
		}
	}

	// delete utxos after that block
	utxos, err := w.DB.Utxos().GetAll()
	if err != nil {
		log.Errorf("unable to resync blockchain: %v", err)
		return
	}
	for _, u := range utxos {
		if u.AtHeight < height {
			// This will catch 0 height unconfirmed utxos as well.
			continue
		}
		if err := w.DB.Utxos().Delete(u); err != nil {
			log.Errorf("error resyncing blockchain: %v", err)
		}
	}

	// delete stxos after that block
	stxos, err := w.DB.Stxos().GetAll()
	if err != nil {
		log.Errorf("unable to resync blockchain: %v", err)
		return
	}
	for _, s := range stxos {
		if s.SpendHeight < height {
			// This will catch 0 height unconfirmed stxos as well.
			continue
		}
		if err := w.DB.Stxos().Delete(s); err != nil {
			log.Errorf("error resyncing blockchain: %v", err)
		}
	}

	w.txStore.PopulateAdrs()

	// reload them all
	w.loadInitialTransactions()
}

// findHeightBeforeTime finds the chain height of the best block before a timestamp
func (w *Wallet) findHeightBeforeTime(ts time.Time) (int32, error) {
	b, err := w.insight.GetBlocksBefore(ts, 1)
	if err != nil || len(b.Blocks) != 1 {
		return 0, err
	}
	return int32(b.Blocks[0].Height), nil
}

// Return the number of confirmations and the height for a transaction
func (w *Wallet) GetConfirmations(txid chainhash.Hash) (confirms, atHeight uint32, err error) {
	txn, err := w.DB.Txns().Get(txid)
	if err != nil || txn.Height == 0 {
		return 0, 0, err
	}
	chainTip, _ := w.ChainTip()
	return chainTip - uint32(txn.Height) + 1, uint32(txn.Height), nil
}

// Cleanly disconnect from the wallet
func (w *Wallet) Close() {
	w.insight.Close()
}
