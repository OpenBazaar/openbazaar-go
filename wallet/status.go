package wallet

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
)

type StatusUpdater struct {
	w   wallet.Wallet
	c   chan repo.Notifier
	ctx context.Context
}

type walletUpdateWrapper struct {
	WalletUpdate walletUpdate `json:"walletUpdate"`
}

type walletUpdate struct {
	Height      uint32 `json:"height"`
	Unconfirmed int64  `json:"unconfirmed"`
	Confirmed   int64  `json:"confirmed"`
}

func NewStatusUpdater(w wallet.Wallet, c chan repo.Notifier, ctx context.Context) *StatusUpdater {
	return &StatusUpdater{w, c, ctx}
}

func (s *StatusUpdater) Start() {
	t := time.NewTicker(time.Second * 15)
	for {
		select {
		case <-t.C:
			confirmed, unconfirmed := s.w.Balance()
			height, _ := s.w.ChainTip()
			u := walletUpdate{
				Height:      height,
				Unconfirmed: unconfirmed,
				Confirmed:   confirmed,
			}
			ser, err := json.MarshalIndent(walletUpdateWrapper{u}, "", "    ")
			if err != nil {
				continue
			}
			s.c <- repo.PremarshalledNotifier{ser}
		case <-s.ctx.Done():
			break
		}
	}
}
