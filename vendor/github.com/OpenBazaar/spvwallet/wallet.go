package spvwallet

import (
	"github.com/btcsuite/btcd/chaincfg"
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
	repoPath string, db Datastore, userAgent string, logger logging.LeveledBackend) *SPVWallet {

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
	w.diconnectChan = make(chan string)
	w.peerGroup = make(map[string]*Peer)
	return w
}

func (w *SPVWallet) Start() {
	w.queryDNSSeeds()

	// setup TxStore first (before spvcon)
	w.state = NewTxStore(w.params, w.db, w.masterPrivateKey)

	// shuffle addrs
	for i := range w.addrs {
		j := rand.Intn(i + 1)
		w.addrs[i], w.addrs[j] = w.addrs[j], w.addrs[i]
	}

	// create header db
	bc := NewBlockchain(w.repoPath, w.params)
	w.blockchain = bc
	//bc.Print()

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

	go w.connectToPeers()
	go w.onPeerDisconnect()
}

// Loop through creating new peers until we reach MAX_PEERS
// If we don't have a download peer set we will set one
func (w *SPVWallet) connectToPeers() {
	for {
		if len(w.peerGroup) < MAX_PEERS {
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
			w.connectToPeers()
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
		if stxo.SpendTxid == utxo.Op.Hash {
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

func (w *SPVWallet) Balance() (confirmed, unconfirmed int64) {
	utxos, _ := w.db.Utxos().GetAll()
	stxos, _ := w.db.Stxos().GetAll()
	for _, utxo := range utxos {
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
	return confirmed, unconfirmed
}

func (w *SPVWallet) Params() *chaincfg.Params {
	return w.params
}
