package resync

import (
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("ResyncManager")

var ResyncInterval = time.Hour

type ResyncManager struct {
	sales repo.SaleStore
	w     wallet.Wallet
}

func NewResyncManager(salesDB repo.SaleStore, w wallet.Wallet) *ResyncManager {
	return &ResyncManager{salesDB, w}
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
	rollbackTime := time.Unix(2147483647, 0)
	for _, uf := range unfunded {
		if uf.Timestamp.Before(rollbackTime) {
			rollbackTime = uf.Timestamp.Add(-time.Hour * 24)
		}
		r.sales.SetNeedsResync(uf.OrderId, false)
	}
	if r.w != nil {
		log.Infof("Rolling back blockchain %s looking for payments for %d orders\n", time.Since(rollbackTime), len(unfunded))
		r.w.ReSyncBlockchain(rollbackTime)
	}
}
