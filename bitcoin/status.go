package bitcoin

import (
	"context"
	"encoding/json"
	"time"
)

type StatusUpdater struct {
	w   BitcoinWallet
	c   chan interface{}
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

func NewStatusUpdater(w BitcoinWallet, c chan interface{}, ctx context.Context) *StatusUpdater {
	return &StatusUpdater{w, c, ctx}
}

func (s *StatusUpdater) Start() {
	t := time.NewTicker(time.Second * 15)
	for {
		select {
		case <-t.C:
			confirmed, unconfirmed := s.w.Balance()
			u := walletUpdate{
				Height:      s.w.ChainTip(),
				Unconfirmed: unconfirmed,
				Confirmed:   confirmed,
			}
			ser, err := json.MarshalIndent(walletUpdateWrapper{u}, "", "    ")
			if err != nil {
				continue
			}
			s.c <- ser
		case <-s.ctx.Done():
			break
		}
	}
}
