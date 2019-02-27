package wallet

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OpenBazaar/multiwallet"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

type StatusUpdater struct {
	mw  multiwallet.MultiWallet
	c   chan repo.Notifier
	ctx context.Context
}

type walletUpdateWrapper struct {
	WalletUpdate map[string]walletUpdate `json:"walletUpdate"`
}

type walletUpdate struct {
	Height      uint32 `json:"height"`
	Unconfirmed int64  `json:"unconfirmed"`
	Confirmed   int64  `json:"confirmed"`
}

func NewStatusUpdater(mw multiwallet.MultiWallet, c chan repo.Notifier, ctx context.Context) *StatusUpdater {
	return &StatusUpdater{mw, c, ctx}
}

func (s *StatusUpdater) Start() {
	t := time.NewTicker(time.Second * 15)
	for {
		select {
		case <-t.C:
			ret := make(map[string]walletUpdate)
			for ct, wal := range s.mw {
				confirmed, unconfirmed := wal.Balance()
				height, _ := wal.ChainTip()
				u := walletUpdate{
					Height:      height,
					Unconfirmed: unconfirmed,
					Confirmed:   confirmed,
				}
				ret[ct.CurrencyCode()] = u
			}
			ser, err := json.MarshalIndent(walletUpdateWrapper{ret}, "", "    ")
			if err != nil {
				continue
			}
			s.c <- repo.PremarshalledNotifier{ser}
		case <-s.ctx.Done():
			break
		}
	}
}
