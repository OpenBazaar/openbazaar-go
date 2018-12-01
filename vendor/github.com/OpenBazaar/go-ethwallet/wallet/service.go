package wallet

import (
	"context"
	"sync"

	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
)

// Service - used to represent WalletService
type Service struct {
	db       wallet.Datastore
	client   *ethclient.Client
	coinType wallet.CoinType

	chainHeight uint32
	bestBlock   string

	lock sync.RWMutex

	doneChan chan struct{}
}

const nullHash = "0000000000000000000000000000000000000000000000000000000000000000"

// NewWalletService - used to create new wallet service
func NewWalletService(db wallet.Datastore, client *ethclient.Client, coinType wallet.CoinType) *Service {
	return &Service{db, client, coinType, 0, nullHash, sync.RWMutex{}, make(chan struct{})}
}

// Start - the wallet daemon
func (ws *Service) Start() {
	log.Infof("Starting %s WalletService", ws.coinType.String())
	go ws.UpdateState()
}

// Stop - the wallet daemon
func (ws *Service) Stop() {
	ws.doneChan <- struct{}{}
}

// ChainTip - get the chain tip
func (ws *Service) ChainTip() (uint32, chainhash.Hash) {
	ws.lock.RLock()
	defer ws.lock.RUnlock()
	ch, _ := chainhash.NewHashFromStr(ws.bestBlock)
	return uint32(ws.chainHeight), *ch
}

// UpdateState - updates state
func (ws *Service) UpdateState() {
	// Start by fetching the chain height from the API
	log.Debugf("querying for %s chain height", ws.coinType.String())
	best, err := ws.client.HeaderByNumber(context.Background(), nil)
	if err == nil {
		log.Debugf("%s chain height: %d", ws.coinType.String(), best.Nonce)
		ws.lock.Lock()
		ws.chainHeight = uint32(best.Number.Uint64())
		ws.bestBlock = best.TxHash.String()
		ws.lock.Unlock()
	} else {
		log.Errorf("error querying API for chain height: %s", err.Error())
	}

}
