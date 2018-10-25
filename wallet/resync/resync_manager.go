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
	sales repo.SaleStore
	mw    multiwallet.MultiWallet
}

func NewResyncManager(salesDB repo.SaleStore, mw multiwallet.MultiWallet) *ResyncManager {
	return &ResyncManager{salesDB, mw}
}

func (r *ResyncManager) Start() {
	t := time.NewTicker(ResyncInterval)
	for ; true; <-t.C {
		r.CheckUnfunded()
	}
}

func (r *ResyncManager) CheckUnfunded() {
	unfunded, err := r.sales.GetNeedsResync()
	if err != nil {
		log.Error(err)
		return
	}
	if len(unfunded) == 0 {
		return
	}
	wallets := make(map[string]time.Time)
	rollbackTime := time.Unix(2147483647, 0)
	if r.mw != nil {
		for cc := range r.mw {
			wallets[strings.ToUpper(cc.CurrencyCode())] = rollbackTime
		}
	}
	for _, uf := range unfunded {
		t, ok := wallets[strings.ToUpper(uf.PaymentCoin)]
		if !ok {
			log.Warningf("ResyncManager: no wallet for sale with payment coin %s", uf.PaymentCoin)
			continue
		}
		if uf.Timestamp.Before(t) {
			t = uf.Timestamp.Add(-time.Hour * 24)
			wallets[strings.ToUpper(uf.PaymentCoin)] = t
		}
		r.sales.SetNeedsResync(uf.OrderId, false)
	}
	if r.mw != nil {
		for cc, rbt := range wallets {
			wal, err := r.mw.WalletForCurrencyCode(cc)
			if err != nil {
				log.Warningf("ResyncManager: no wallet for sale with payment coin %s", cc)
				continue
			}
			log.Infof("Rolling back %s blockchain %s looking for payments for %d orders\n", cc, time.Since(rbt), len(unfunded))
			wal.ReSyncBlockchain(rbt)
		}
	}
}
