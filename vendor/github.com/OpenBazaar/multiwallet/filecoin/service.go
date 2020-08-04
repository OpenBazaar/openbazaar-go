package filecoin

import (
	"encoding/json"
	"fmt"
	"github.com/OpenBazaar/multiwallet/cache"
	"github.com/OpenBazaar/multiwallet/model"
	"github.com/OpenBazaar/multiwallet/service"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/ipfs/go-cid"
	"github.com/op/go-logging"
	"math/big"
	"sync"
	"time"
)

var Log = logging.MustGetLogger("Filecoin")

type FilecoinService struct {
	db       wallet.Datastore
	addr     btcutil.Address
	client   model.APIClient
	params   *chaincfg.Params
	coinType wallet.CoinType

	chainHeight uint32
	bestBlock   string
	cache       cache.Cacher

	listeners []func(wallet.TransactionCallback)

	lock sync.RWMutex

	doneChan chan struct{}
}

func NewFilecoinService(db wallet.Datastore, addr btcutil.Address, client model.APIClient, params *chaincfg.Params, coinType wallet.CoinType, cache cache.Cacher) (*FilecoinService, error) {
	fs := &FilecoinService{
		db:       db,
		addr:     addr,
		client:   client,
		params:   params,
		coinType: coinType,
		cache:    cache,
		doneChan: make(chan struct{}),
		lock:     sync.RWMutex{},
	}
	marshaledHeight, err := cache.Get(fs.bestHeightKey())
	if err != nil {
		Log.Info("cached block height missing: using default")
	} else {
		var hh service.HashAndHeight
		if err := json.Unmarshal(marshaledHeight, &hh); err != nil {
			Log.Error("failed unmarshaling cached block height")
			return fs, nil
		}
		fs.bestBlock = hh.Hash
		fs.chainHeight = hh.Height
	}
	return fs, nil
}

func (fs *FilecoinService) Start() {
	Log.Noticef("starting FilecoinService")
	go fs.run()
}

func (fs *FilecoinService) run() {
	var (
		txChan    = fs.client.TransactionNotify()
		blockChan = fs.client.BlockNotify()
	)

	fs.client.ListenAddresses(fs.addr)

	for {
		select {
		case <-fs.doneChan:
			return
		case tx := <-txChan:
			go fs.ProcessIncomingTransaction(tx)
		case block := <-blockChan:
			go fs.processIncomingBlock(block)
		}
	}
}

func (fs *FilecoinService) Stop() {
	close(fs.doneChan)
}

func (fs *FilecoinService) ChainTip() (uint32, string) {
	fs.lock.RLock()
	defer fs.lock.RUnlock()
	return fs.chainHeight, fs.bestBlock
}

func (fs *FilecoinService) AddTransactionListener(callback func(callback wallet.TransactionCallback)) {
	fs.listeners = append(fs.listeners, callback)
}

func (fs *FilecoinService) InvokeTransactionListeners(callback wallet.TransactionCallback) {
	for _, l := range fs.listeners {
		go l(callback)
	}
}

// This is a transaction fresh off the wire. Let's save it to the db.
func (fs *FilecoinService) ProcessIncomingTransaction(tx model.Transaction) {
	Log.Debugf("new incoming %s transaction: %s", fs.coinType.String(), tx.Txid)

	fs.lock.RLock()
	chainHeight := int32(fs.chainHeight)
	fs.lock.RUnlock()
	fs.saveSingleTxToDB(tx, chainHeight)
}

func (fs *FilecoinService) UpdateState() {
	// Start by fetching the chain height from the API
	Log.Debugf("updating %s chain state", fs.coinType.String())
	best, err := fs.client.GetBestBlock()
	if err == nil {
		Log.Debugf("%s chain height: %d", fs.coinType.String(), best.Height)
		fs.lock.Lock()
		err = fs.saveHashAndHeight(best.Hash, uint32(best.Height))
		if err != nil {
			Log.Errorf("updating %s blockchain height: %s", fs.coinType.String(), err.Error())
		}
		fs.lock.Unlock()
	} else {
		Log.Errorf("error querying API for %s chain height: %s", fs.coinType.String(), err.Error())
	}

	go fs.syncTxs()
}

func (fs *FilecoinService) syncTxs() {
	Log.Debugf("querying for %s transactions", fs.coinType.String())
	query := []btcutil.Address{fs.addr}
	txs, err := fs.client.GetTransactions(query)
	if err != nil {
		Log.Errorf("error downloading txs for %s: %s", fs.coinType.String(), err.Error())
	} else {
		Log.Debugf("downloaded %d %s transactions", len(txs), fs.coinType.String())
		fs.lock.RLock()
		chainHeight := int32(fs.chainHeight)
		fs.lock.RUnlock()
		for _, u := range txs {
			fs.saveSingleTxToDB(u, chainHeight)
		}
	}
}

func (fs *FilecoinService) processIncomingBlock(block model.Block) {
	Log.Infof("received new %s block at height %d: %s", fs.coinType.String(), block.Height, block.Hash)
	fs.lock.RLock()
	currentBest := fs.bestBlock
	fs.lock.RUnlock()

	fs.lock.Lock()
	err := fs.saveHashAndHeight(block.Hash, uint32(block.Height))
	if err != nil {
		Log.Errorf("update %s blockchain height: %s", fs.coinType.String(), err.Error())
	}
	fs.lock.Unlock()

	// REORG! Rescan all transactions and utxos to see if anything changed
	if currentBest != block.PreviousBlockhash && currentBest != block.Hash {
		Log.Warningf("%s chain reorg detected: rescanning wallet", fs.coinType.String())
		fs.UpdateState()
		return
	}

	// Query db for unconfirmed txs and utxos then query API to get current height
	txs, err := fs.db.Txns().GetAll(true)
	if err != nil {
		Log.Errorf("error loading %s txs from db: %s", fs.coinType.String(), err.Error())
		return
	}
	for _, tx := range txs {
		if tx.Height == 0 {
			Log.Debugf("broadcasting unconfirmed txid %s", tx.Txid)
			go func(txn wallet.Txn) {
				ret, err := fs.client.GetTransaction(txn.Txid)
				if err != nil {
					Log.Errorf("error fetching unconfirmed %s tx: %s", fs.coinType.String(), err.Error())
					return
				}
				if ret.Confirmations > 0 {
					fs.saveSingleTxToDB(*ret, int32(block.Height))
					return
				}
				// Incoming txs do not have a signature attached and we can't rebroadcast.
				if txn.Value != "" {
					bigVal, _ := new(big.Int).SetString(txn.Value, 10)
					if bigVal.Cmp(big.NewInt(0)) > 0 {
						return
					}
				}
				// Rebroadcast unconfirmed transactions
				_, err = fs.client.Broadcast(tx.Bytes)
				if err != nil {
					Log.Errorf("broadcasting unconfirmed utxo: %s", err.Error())
				}
			}(tx)
		}
	}
}

func (fs *FilecoinService) saveSingleTxToDB(u model.Transaction, chainHeight int32) {
	hits := 0
	value := big.NewInt(0)

	height := int32(0)
	if u.Confirmations > 0 {
		height = chainHeight - (int32(u.Confirmations) - 1)
	}

	txHash, err := cid.Decode(u.Txid)

	if err != nil {
		Log.Errorf("error converting to txHash for %s: %s", fs.coinType.String(), err.Error())
		return
	}
	var relevant bool
	sender := false
	cb := wallet.TransactionCallback{Txid: txHash.String(), Height: height, Timestamp: time.Unix(u.Time, 0)}

	for _, in := range u.Inputs {
		faddr, err := NewFilecoinAddress(in.Addr)
		if err != nil {
			Log.Errorf("error parsing address %s: %s", fs.coinType.String(), err.Error())
			continue
		}

		if in.ValueIface == nil {
			if in.Addr == fs.addr.String() {
				relevant = true
				sender = true
				hits++
			}
			continue
		}

		v, _ := new(big.Int).SetString(in.ValueIface.(string), 10)
		cbin := wallet.TransactionInput{
			LinkedAddress: faddr,
			Value:         *v,
		}
		cb.Inputs = append(cb.Inputs, cbin)

		if in.Addr == fs.addr.String() {
			relevant = true
			sender = true
			hits++
		}
	}
	for i, out := range u.Outputs {
		if len(out.ScriptPubKey.Addresses) == 0 {
			continue
		}
		faddr, err := NewFilecoinAddress(out.ScriptPubKey.Addresses[0])
		if err != nil {
			Log.Errorf("error parsing address %s: %s", fs.coinType.String(), err.Error())
			continue
		}

		v, _ := new(big.Int).SetString(out.ValueIface.(string), 10)

		cbout := wallet.TransactionOutput{Address: faddr, Value: *v, Index: uint32(i)}
		cb.Outputs = append(cb.Outputs, cbout)

		if out.ScriptPubKey.Addresses[0] == fs.addr.String() {
			relevant = true
			hits++
		}

		if sender {
			value.Sub(value, v)
		} else {
			value.Add(value, v)
		}
	}

	if value.String() == "0" {
		relevant = false
	}

	if !relevant {
		Log.Warningf("abort saving irrelevant txid (%s) to db", u.Txid)
		return
	}

	cb.Value = *value

	saved, err := fs.db.Txns().Get(txHash.String())
	if err != nil {

		// Check to see if this is a incoming block tx
		allTxs, err := fs.db.Txns().GetAll(true)
		for _, iTx := range allTxs {
			sm, checkError := types.DecodeSignedMessage(iTx.Bytes)
			if checkError != nil {
				continue
			}
			if sm.Message.Cid().String() == u.Txid {
				fmt.Println("found match")
				txHash = sm.Cid()
				u.RawBytes = iTx.Bytes
				break
			}
		}

		ts := time.Now()
		if u.Confirmations > 0 {
			ts = time.Unix(u.BlockTime, 0)
		}
		err = fs.db.Txns().Put(u.RawBytes, txHash.String(), value.String(), int(height), ts, hits == 0)
		if err != nil {
			Log.Errorf("putting txid (%s): %s", txHash.String(), err.Error())
			return
		}
		cb.Timestamp = ts
		fs.callbackListeners(cb)
	} else if height > 0 {
		err := fs.db.Txns().UpdateHeight(txHash.String(), int(height), time.Unix(u.BlockTime, 0))
		if err != nil {
			Log.Errorf("updating height for tx (%s): %s", txHash.String(), err.Error())
			return
		}
		if saved.Height != height {
			cb.Timestamp = saved.Timestamp
			fs.callbackListeners(cb)
		}
	}
}

func (fs *FilecoinService) callbackListeners(cb wallet.TransactionCallback) {
	for _, callback := range fs.listeners {
		callback(cb)
	}
}

func (fs *FilecoinService) saveHashAndHeight(hash string, height uint32) error {
	hh := service.HashAndHeight{
		Height:    height,
		Hash:      hash,
		Timestamp: time.Now(),
	}
	b, err := json.MarshalIndent(&hh, "", "    ")
	if err != nil {
		return err
	}
	fs.chainHeight = height
	fs.bestBlock = hash
	return fs.cache.Set(fs.bestHeightKey(), b)
}

func (fs *FilecoinService) bestHeightKey() string {
	return fmt.Sprintf("best-height-%s", fs.coinType.String())
}
