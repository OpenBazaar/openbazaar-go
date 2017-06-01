package spvwallet

import (
	"errors"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/txscript"
	btc "github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/op/go-logging"
	b39 "github.com/tyler-smith/go-bip39"
	"os"
	"path"
	"sync"
)

type SPVWallet struct {
	params *chaincfg.Params

	masterPrivateKey *hd.ExtendedKey
	masterPublicKey  *hd.ExtendedKey

	maxFee      uint64
	priorityFee uint64
	normalFee   uint64
	economicFee uint64
	feeAPI      string

	repoPath string

	blockchain  *Blockchain
	txstore     *TxStore
	peerManager *PeerManager
	keyManager  *KeyManager

	fPositives    chan *peer.Peer
	stopChan      chan int
	fpAccumulator map[int32]int32
	blockQueue    chan chainhash.Hash
	mutex         *sync.RWMutex

	running bool

	config *PeerManagerConfig
}

var log = logging.MustGetLogger("bitcoin")

const WALLET_VERSION = "0.1.0"

func NewSPVWallet(config *Config) (*SPVWallet, error) {

	log.SetBackend(logging.AddModuleLevel(config.Logger))

	if config.Mnemonic == "" {
		ent, err := b39.NewEntropy(128)
		if err != nil {
			return nil, err
		}
		mnemonic, err := b39.NewMnemonic(ent)
		if err != nil {
			return nil, err
		}
		config.Mnemonic = mnemonic
	}
	seed := b39.NewSeed(config.Mnemonic, "")

	mPrivKey, err := hd.NewMaster(seed, config.Params)
	if err != nil {
		return nil, err
	}
	mPubKey, err := mPrivKey.Neuter()
	if err != nil {
		return nil, err
	}
	w := &SPVWallet{
		repoPath:         config.RepoPath,
		masterPrivateKey: mPrivKey,
		masterPublicKey:  mPubKey,
		params:           config.Params,
		maxFee:           config.MaxFee,
		priorityFee:      config.HighFee,
		normalFee:        config.MediumFee,
		economicFee:      config.LowFee,
		feeAPI:           config.FeeAPI.String(),
		fPositives:       make(chan *peer.Peer),
		stopChan:         make(chan int),
		fpAccumulator:    make(map[int32]int32),
		blockQueue:       make(chan chainhash.Hash, 32),
		mutex:            new(sync.RWMutex),
	}

	w.keyManager, err = NewKeyManager(config.DB.Keys(), w.params, w.masterPrivateKey)

	w.txstore, err = NewTxStore(w.params, config.DB, w.keyManager)
	if err != nil {
		return nil, err
	}

	w.blockchain, err = NewBlockchain(w.repoPath, w.params)
	if err != nil {
		return nil, err
	}

	listeners := &peer.MessageListeners{
		OnMerkleBlock: w.onMerkleBlock,
		OnInv:         w.onInv,
		OnTx:          w.onTx,
		OnGetData:     w.onGetData,
	}

	getNewestBlock := func() (*chainhash.Hash, int32, error) {
		storedHeader, err := w.blockchain.db.GetBestHeader()
		if err != nil {
			return nil, 0, err
		}
		height, err := w.blockchain.db.Height()
		if err != nil {
			return nil, 0, err
		}
		hash := storedHeader.header.BlockHash()
		return &hash, int32(height), nil
	}

	w.config = &PeerManagerConfig{
		UserAgentName:      config.UserAgent,
		UserAgentVersion:   WALLET_VERSION,
		Params:             w.params,
		AddressCacheDir:    config.RepoPath,
		GetFilter:          w.txstore.GimmeFilter,
		StartChainDownload: w.startChainDownload,
		GetNewestBlock:     getNewestBlock,
		Listeners:          listeners,
		Proxy:              config.Proxy,
	}

	if config.TrustedPeer != nil {
		w.config.TrustedPeer = config.TrustedPeer
	}

	w.peerManager, err = NewPeerManager(w.config)
	if err != nil {
		return nil, err
	}

	return w, nil
}

func (w *SPVWallet) Start() {
	w.running = true
	go w.peerManager.Start()
	w.fPositiveHandler(w.stopChan)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// API
//
//////////////

func (w *SPVWallet) CurrencyCode() string {
	if w.params.Name == chaincfg.MainNetParams.Name {
		return "btc"
	} else {
		return "tbtc"
	}
}

func (w *SPVWallet) MasterPrivateKey() *hd.ExtendedKey {
	return w.masterPrivateKey
}

func (w *SPVWallet) MasterPublicKey() *hd.ExtendedKey {
	return w.masterPublicKey
}

func (w *SPVWallet) ConnectedPeers() []*peer.Peer {
	return w.peerManager.ConnectedPeers()
}

func (w *SPVWallet) CurrentAddress(purpose KeyPurpose) btc.Address {
	key, _ := w.keyManager.GetCurrentKey(purpose)
	addr, _ := key.Address(w.params)
	return btc.Address(addr)
}

func (w *SPVWallet) NewAddress(purpose KeyPurpose) btc.Address {
	i, _ := w.txstore.Keys().GetUnused(purpose)
	key, _ := w.keyManager.generateChildKey(purpose, uint32(i[1]))
	addr, _ := key.Address(w.params)
	script, _ := txscript.PayToAddrScript(btc.Address(addr))
	w.txstore.Keys().MarkKeyAsUsed(script)
	w.txstore.PopulateAdrs()
	return btc.Address(addr)
}

func (w *SPVWallet) DecodeAddress(addr string) (btc.Address, error) {
	return btc.DecodeAddress(addr, w.params)
}

func (w *SPVWallet) ScriptToAddress(script []byte) (btc.Address, error) {
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(script, w.params)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, errors.New("unknown script")
	}
	return addrs[0], nil
}

func (w *SPVWallet) AddressToScript(addr btc.Address) ([]byte, error) {
	return txscript.PayToAddrScript(addr)
}

func (w *SPVWallet) HasKey(addr btc.Address) bool {
	script, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return false
	}
	_, err = w.keyManager.GetKeyForScript(script)
	if err != nil {
		return false
	}
	return true
}

func (w *SPVWallet) Balance() (confirmed, unconfirmed int64) {
	utxos, _ := w.txstore.Utxos().GetAll()
	stxos, _ := w.txstore.Stxos().GetAll()
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

func (w *SPVWallet) Transactions() ([]Txn, error) {
	return w.txstore.Txns().GetAll(false)
}

func (w *SPVWallet) GetTransaction(txid chainhash.Hash) (Txn, error) {
	_, txn, err := w.txstore.Txns().Get(txid)
	return txn, err
}

func (w *SPVWallet) GetConfirmations(txid chainhash.Hash) (uint32, error) {
	_, txn, err := w.txstore.Txns().Get(txid)
	if err != nil {
		return 0, err
	}
	if txn.Height == 0 {
		return 0, nil
	}
	chainTip := w.ChainTip()
	return chainTip - uint32(txn.Height), nil
}

func (w *SPVWallet) checkIfStxoIsConfirmed(utxo Utxo, stxos []Stxo) bool {
	for _, stxo := range stxos {
		if stxo.SpendTxid.IsEqual(&utxo.Op.Hash) {
			if stxo.SpendHeight > 0 {
				return true
			} else {
				return w.checkIfStxoIsConfirmed(stxo.Utxo, stxos)
			}
		}
	}
	return false
}

func (w *SPVWallet) Params() *chaincfg.Params {
	return w.params
}

func (w *SPVWallet) AddTransactionListener(callback func(TransactionCallback)) {
	w.txstore.listeners = append(w.txstore.listeners, callback)
}

func (w *SPVWallet) ChainTip() uint32 {
	height, _ := w.blockchain.db.Height()
	return uint32(height)
}

func (w *SPVWallet) AddWatchedScript(script []byte) error {
	err := w.txstore.WatchedScripts().Put(script)
	w.txstore.PopulateAdrs()

	for _, peer := range w.peerManager.ConnectedPeers() {
		w.updateFilterAndSend(peer)
	}
	return err
}

func (w *SPVWallet) GenerateMultisigScript(keys []hd.ExtendedKey, threshold int) (addr btc.Address, redeemScript []byte, err error) {
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

func (w *SPVWallet) Close() {
	if w.running {
		log.Info("Disconnecting from peers and shutting down")
		w.peerManager.Stop()
		w.blockchain.Close()
		w.stopChan <- 1
		w.running = false
	}
}

func (w *SPVWallet) ReSyncBlockchain(fromHeight int32) {
	w.Close()
	os.Remove(path.Join(w.repoPath, "headers.bin"))
	blockchain, err := NewBlockchain(w.repoPath, w.params)
	if err != nil {
		return
	}
	w.blockchain = blockchain
	w.peerManager, err = NewPeerManager(w.config)
	if err != nil {
		return
	}
	w.blockQueue = make(chan chainhash.Hash, 32)
	go w.Start()
}
