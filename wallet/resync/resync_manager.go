package resync

import (
	"strings"
	"time"

	"github.com/btcsuite/btcutil"

	"github.com/OpenBazaar/multiwallet"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("ResyncManager")

var ResyncInterval = time.Minute * 15

type ResyncManager struct {
	sales     repo.SaleStore
	purchases repo.PurchaseStore
	mw        multiwallet.MultiWallet
}

func NewResyncManager(salesDB repo.SaleStore, purchaseDB repo.PurchaseStore, mw multiwallet.MultiWallet) *ResyncManager {
	return &ResyncManager{sales: salesDB, purchases: purchaseDB, mw: mw}
}

func (r *ResyncManager) Start() {
	t := time.NewTicker(ResyncInterval)
	for ; true; <-t.C {
		r.CheckUnfunded()
	}
}

func (r *ResyncManager) CheckUnfunded() {
	unfundedSales, err := r.sales.GetUnfunded()
	if err != nil {
		log.Error(err)
		return
	}
	unfundedPurchases, err := r.purchases.GetUnfunded()
	if err != nil {
		log.Error(err)
		return
	}
	unfunded := append(unfundedSales, unfundedPurchases...)
	if len(unfunded) == 0 {
		return
	}
	wallets := make(map[string][]string)
	for _, uf := range unfunded {
		addrs, ok := wallets[strings.ToUpper(uf.PaymentCoin)]
		if !ok {
			addrs = []string{}
		}
		addrs = append(addrs, uf.PaymentAddress)
		wallets[strings.ToUpper(uf.PaymentCoin)] = addrs
	}
	if r.mw != nil {
		for cc, addrs := range wallets {
			wal, err := r.mw.WalletForCurrencyCode(cc)
			if err != nil {
				log.Warningf("ResyncManager: no wallet for sale with payment coin %s", cc)
				continue
			}

			var decodedAddresses []btcutil.Address
			for _, addr := range addrs {
				iaddr, err := wal.DecodeAddress(addr)
				if err != nil {
					log.Errorf("Error decoding unfunded payment address(%s): %s", addr, err)
					continue
				}

				decodedAddresses = append(decodedAddresses, iaddr)
			}

			err = wal.AddWatchedAddresses(decodedAddresses...)
			if err != nil {
				log.Warningf("ResyncManager: couldn't add watched addresses for coin: %s", cc)
				continue
			}

			log.Infof("Rescanning %s wallet looking for %d orders", cc, len(unfunded))
			wal.ReSyncBlockchain(time.Time{})
		}
	}
}
