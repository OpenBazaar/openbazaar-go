package resync

import (
	"strings"
	"time"

	"github.com/OpenBazaar/multiwallet"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("ResyncManager")

var ResyncInterval = time.Hour

type ResyncManager struct {
	sales     repo.SaleStore
	purchases repo.PurchaseStore
	mw        multiwallet.MultiWallet
}

func NewResyncManager(salesDB repo.SaleStore, purchaseDB repo.PurchaseStore, mw multiwallet.MultiWallet) *ResyncManager {
	return &ResyncManager{salesDB, purchaseDB, mw}
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
			wallets[strings.ToUpper(uf.PaymentCoin)] = addrs
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
			for _, addr := range addrs {
				iaddr, err := wal.DecodeAddress(addr)
				if err != nil {
					log.Errorf("Error decoding unfunded payment address: %s", err)
					continue
				}
				if err := wal.AddWatchedAddress(iaddr); err != nil {
					log.Errorf("Error adding watched address: %s", err)
				}
			}

			log.Infof("Rescanning blocking for %d orders\n", cc, len(unfunded))
			wal.ReSyncBlockchain(time.Time{})
		}
	}
}
