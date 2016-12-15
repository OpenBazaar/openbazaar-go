package spvwallet

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	btc "github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/op/go-logging"
	b39 "github.com/tyler-smith/go-bip39"
	"math/rand"
	"net"
	"sync"
)

type SPVWallet struct {
	params *chaincfg.Params

	peerGroup     map[string]*Peer
	pgMutex       sync.Mutex
	downloadPeer  *Peer
	trustedPeer   string
	diconnectChan chan string

	masterPrivateKey *hd.ExtendedKey
	masterPublicKey  *hd.ExtendedKey

	maxFee      uint64
	priorityFee uint64
	normalFee   uint64
	economicFee uint64
	feeAPI      string

	repoPath string

	addrs     []string
	userAgent string

	db         Datastore
	blockchain *Blockchain
	state      *TxStore
}

var log = logging.MustGetLogger("bitcoin")

const WALLET_VERSION = "0.1.0"

const MAX_PEERS = 10

func NewSPVWallet(mnemonic string, params *chaincfg.Params, maxFee uint64, lowFee uint64, mediumFee uint64, highFee uint64, feeApi,
	repoPath string, db Datastore, userAgent string, trustedPeer string, logger logging.LeveledBackend) *SPVWallet {

	log.SetBackend(logger)

	seed := b39.NewSeed(mnemonic, "")

	mPrivKey, _ := hd.NewMaster(seed, params)
	mPubKey, _ := mPrivKey.Neuter()

	w := new(SPVWallet)
	w.masterPrivateKey = mPrivKey
	w.masterPublicKey = mPubKey
	w.params = params
	w.maxFee = maxFee
	w.priorityFee = highFee
	w.normalFee = mediumFee
	w.economicFee = lowFee
	w.feeAPI = feeApi
	w.repoPath = repoPath
	w.db = db
	w.userAgent = userAgent
	w.trustedPeer = trustedPeer
	w.diconnectChan = make(chan string)
	w.peerGroup = make(map[string]*Peer)
	w.state = NewTxStore(w.params, w.db, w.masterPrivateKey)
	return w
}

func (w *SPVWallet) Start() {

	if w.trustedPeer == "" {
		w.queryDNSSeeds()
	}

	// shuffle addrs
	for i := range w.addrs {
		j := rand.Intn(i + 1)
		w.addrs[i], w.addrs[j] = w.addrs[j], w.addrs[i]
	}

	// create header db
	bc := NewBlockchain(w.repoPath, w.params)
	w.blockchain = bc
	//bc.db.Print()

	// If this is a new wallet or restoring from seed. Set the db height to the
	// height of the checkpoint block.
	tipHeight, _ := w.state.GetDBSyncHeight()
	if tipHeight == 0 {
		if w.params.Name == chaincfg.MainNetParams.Name {
			w.state.SetDBSyncHeight(MAINNET_CHECKPOINT_HEIGHT)
		} else if w.params.Name == chaincfg.TestNet3Params.Name {
			w.state.SetDBSyncHeight(TESTNET3_CHECKPOINT_HEIGHT)
		}
	}

	if w.trustedPeer == "" {
		go w.connectToPeers()
	} else {
		peer, err := NewPeer(w.trustedPeer, w.blockchain, w.state, w.params, w.userAgent, w.diconnectChan, true)
		if err != nil {
			log.Fatal("Failed to connect to trusted peer")
		}
		w.downloadPeer = peer
		w.peerGroup[w.trustedPeer] = peer
	}
	go w.onPeerDisconnect()
}

// Loop through creating new peers until we reach MAX_PEERS
// If we don't have a download peer set we will set one
func (w *SPVWallet) connectToPeers() {
	for {
		if len(w.peerGroup) < MAX_PEERS && len(w.addrs) > 0 {
			var addr string
			addr, w.addrs = w.addrs[len(w.addrs)-1], w.addrs[:len(w.addrs)-1]
			var dp bool
			if w.downloadPeer == nil {
				dp = true
				// Set this temporarily to avoid a race condition which sets two download peers
				w.downloadPeer = &Peer{}
			}
			peer, err := NewPeer(addr, w.blockchain, w.state, w.params, w.userAgent, w.diconnectChan, dp)
			if err != nil {
				if dp {
					// Unset as download peer on failure
					w.downloadPeer = nil
				}
				continue
			}
			if dp {
				w.downloadPeer = peer
			}
			w.pgMutex.Lock()
			w.peerGroup[addr] = peer
			w.pgMutex.Unlock()
		} else {
			break
		}
	}
}

func (w *SPVWallet) onPeerDisconnect() {
	for {
		select {
		case addr := <-w.diconnectChan:
			w.pgMutex.Lock()
			p, ok := w.peerGroup[addr]
			if ok {
				p.con.Close()
				p.connectionState = DEAD
				if p.downloadPeer {
					w.downloadPeer = nil
				}
				delete(w.peerGroup, addr)
			}
			w.pgMutex.Unlock()
			log.Infof("Disconnected from peer %s", addr)
			if w.trustedPeer == "" {
				w.connectToPeers()
			}
		}
	}
}

func (w *SPVWallet) queryDNSSeeds() {
	// Query DNS seeds for addrs. Eventually we will cache these.
	log.Info("Querying DNS seeds...")
	for _, seed := range w.params.DNSSeeds {
		addrs, err := net.LookupHost(seed)
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			w.addrs = append(w.addrs, addr+":"+w.params.DefaultPort)
		}
	}
	log.Infof("DNS seeds returned %d addresses.", len(w.addrs))
}

func (w *SPVWallet) checkIfStxoIsConfirmed(utxo Utxo, stxos []Stxo) bool {
	for _, stxo := range stxos {
		if stxo.SpendTxid.IsEqual(&utxo.Op.Hash) {
			if stxo.Utxo.AtHeight > 0 {
				return true
			} else {
				return w.checkIfStxoIsConfirmed(stxo.Utxo, stxos)
			}
		}
	}
	return false
}

//////////////////////////
//
// API
//

// A TransactionCallback which is sent from the wallet implementation to the transaction
// listener. It contains enough data to tell which part of the transaction affects our
// wallet and which addresses coins were sent to and from.

type FeeLevel int

const (
	PRIOIRTY FeeLevel = 0
	NORMAL            = 1
	ECONOMIC          = 2
)

type KeyPurpose int

const (
	EXTERNAL KeyPurpose = 0
	INTERNAL            = 1
)

type TransactionCallback struct {
	Txid    []byte
	Outputs []TransactionOutput
	Inputs  []TransactionInput
}

type TransactionOutput struct {
	ScriptPubKey []byte
	Value        int64
	Index        uint32
}

type TransactionInput struct {
	OutpointHash       []byte
	OutpointIndex      uint32
	LinkedScriptPubKey []byte
	Value              int64
}

// A transaction suitable for saving in the database
type TransactionRecord struct {
	Txid         string
	Index        uint32
	Value        int64
	ScriptPubKey string
	Spent        bool
}

type Signature struct {
	InputIndex uint32
	Signature  []byte
}

func (w *SPVWallet) CurrencyCode() string {
	return "btc"
}

func (w *SPVWallet) MasterPrivateKey() *hd.ExtendedKey {
	return w.masterPrivateKey
}

func (w *SPVWallet) MasterPublicKey() *hd.ExtendedKey {
	return w.masterPublicKey
}

func (w *SPVWallet) CurrentAddress(purpose KeyPurpose) btc.Address {
	key := w.state.GetCurrentKey(purpose)
	addr, _ := key.Address(w.params)
	return btc.Address(addr)
}

func (w *SPVWallet) HasKey(addr btc.Address) bool {
	script, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return false
	}
	_, err = w.state.GetKeyForScript(script)
	if err != nil {
		return false
	}
	return true
}

func (w *SPVWallet) Balance() (confirmed, unconfirmed int64) {
	utxos, _ := w.db.Utxos().GetAll()
	stxos, _ := w.db.Stxos().GetAll()
	for _, utxo := range utxos {
		if !utxo.Freeze {
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

func (w *SPVWallet) Params() *chaincfg.Params {
	return w.params
}

func (w *SPVWallet) AddTransactionListener(callback func(TransactionCallback)) {
	w.state.listeners = append(w.state.listeners, callback)
}

func (w *SPVWallet) ChainTip() uint32 {
	height, _ := w.state.GetDBSyncHeight()
	return uint32(height)
}

func (w *SPVWallet) AddWatchedScript(script []byte) error {
	err := w.state.db.WatchedScripts().Put(script)
	w.state.PopulateAdrs()
	for _, peer := range w.peerGroup {
		peer.UpdateFilterAndSend()
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
	log.Info("Disconnecting from peers and shutting down")
	for _, peer := range w.peerGroup {
		peer.con.Close()
		log.Debugf("Disconnnected from %s", peer.con.RemoteAddr().String())
	}
	w.blockchain.Close()
}

func (w *SPVWallet) ReSyncBlockchain(fromHeight int32) {
	w.Close()
	if w.params.Name == chaincfg.MainNetParams.Name && fromHeight < MAINNET_CHECKPOINT_HEIGHT {
		fromHeight = MAINNET_CHECKPOINT_HEIGHT
	} else if w.params.Name == chaincfg.TestNet3Params.Name && fromHeight < TESTNET3_CHECKPOINT_HEIGHT {
		fromHeight = TESTNET3_CHECKPOINT_HEIGHT
	}
	w.state.SetDBSyncHeight(fromHeight)
	go w.Start()
}
