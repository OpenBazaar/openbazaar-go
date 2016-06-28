package libbitcoin

import (
	"time"
	"github.com/OpenBazaar/go-libbitcoinclient"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/op/go-logging"
	b32 "github.com/tyler-smith/go-bip32"
	b39 "github.com/tyler-smith/go-bip39"
	btc "github.com/btcsuite/btcutil"
)

var log = logging.MustGetLogger("LibitcoinWallet")

type LibbitcoinWallet struct {
	Client *libbitcoin.LibbitcoinClient

	Params *chaincfg.Params

	masterPrivateKey *b32.Key
	masterPublicKey  *b32.Key

	db repo.Datastore
}

func NewLibbitcoinWallet(mnemonic string, params *chaincfg.Params, db repo.Datastore, servers []libbitcoin.Server) *LibbitcoinWallet {
	seed := b39.NewSeed(mnemonic, "")
	mk, _ := b32.NewMasterKey(seed)
	l := new(LibbitcoinWallet)
	l.masterPrivateKey = mk
	l.masterPublicKey = mk.PublicKey()
	l.Params = params
	l.Client = libbitcoin.NewLibbitcoinClient(servers, params)
	l.db = db
	go l.startUpdateLoop()
	go l.subscribeAll()
	return l
}

// Calls updateWalletBalances() once at start up and then every 24 hours after.
// In theory we should not need to repeat this call but if the libbitcoin server isn't
// correctly returning Subscribe data, then we will pick up missing transactions
// when this loops through.
func (w *LibbitcoinWallet) startUpdateLoop() {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	go w.updateWalletBalances()
	for {
		select {
		case <-tick.C:
			go w.updateWalletBalances()
		}
	}
}

// Loop through each address in the wallet and fetch the history from the libbitcoin server.
// For each returned txid, fetch the full transaction, checking the mempool first then the blockchain.
// If a transaction is returned well will parse it and check to see if we need to update our wallet state.
func (w *LibbitcoinWallet) updateWalletBalances() {
	keys, _ := w.db.Keys().GetAllExternal()
	for _, k := range(keys) {
		addr, _ := btc.NewAddressPubKey(k.PublicKey().Key, w.Params)
		// FIXME: we don't want to fetch from height zero every time. Ideally it would use the height of the last
		// FIXME: seen block but to handle cases where the server failed to send a transaction we should probably
		// FIXME: use the last height of any transaction in the database ― which requires another db function.
		w.Client.FetchHistory2(addr.AddressPubKeyHash(), 0, func(i interface{}, err error){
			for _, response := range(i.([]libbitcoin.FetchHistory2Resp)) {
				w.Client.FetchUnconfirmedTransaction(response.TxHash, func(i interface{}, err error){
					if err != nil {
						w.Client.FetchTransaction(response.TxHash, func(i interface{}, err error) {
							if err != nil {
								log.Error(err.Error())

							} else {
								tx := i.(*btc.Tx)
								w.ProcessTransaction(tx, response.Height)
							}
						})
					} else {
						tx := i.(*btc.Tx)
						w.ProcessTransaction(tx, response.Height)
					}
				})
			}
		})
	}
}

func (w *LibbitcoinWallet) subscribeAll() {
	keys, _ := w.db.Keys().GetAllExternal()
	for _, k := range(keys) {
		addr, _ := btc.NewAddressPubKey(k.PublicKey().Key, w.Params)
		w.Client.SubscribeAddress(addr.AddressPubKeyHash(), func(i interface{}){
			resp := i.(libbitcoin.SubscribeResp)
			w.ProcessTransaction(&resp.Tx, resp.Height)
		})
	}
}

func (w *LibbitcoinWallet) SubscribeAddress(addr btc.Address) {
	w.Client.SubscribeAddress(addr, func(i interface{}){
		resp := i.(libbitcoin.SubscribeResp)
		w.ProcessTransaction(&resp.Tx, resp.Height)
	})
}