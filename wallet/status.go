package wallet

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OpenBazaar/multiwallet"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/op/go-logging"
)

type StatusUpdater struct {
	mw  multiwallet.MultiWallet
	c   chan repo.Notifier
	ctx context.Context
	log *logging.Logger
}

type walletUpdateWrapper struct {
	WalletUpdate map[string]walletUpdate `json:"walletUpdate"`
}

type walletUpdate struct {
	Height      uint32                   `json:"height"`
	Unconfirmed string                   `json:"unconfirmed"`
	Confirmed   string                   `json:"confirmed"`
	Currency    *repo.CurrencyDefinition `json:"currency"`
}

func NewStatusUpdater(mw multiwallet.MultiWallet, c chan repo.Notifier, ctx context.Context) *StatusUpdater {
	var log = logging.MustGetLogger("walletStatus")
	return &StatusUpdater{
		mw:  mw,
		c:   c,
		ctx: ctx,
		log: log,
	}
}

func (s *StatusUpdater) Start() {
	var (
		t = time.NewTicker(time.Second * 15)
	)

	for {
		select {
		case <-t.C:
			ret := make(map[string]walletUpdate)
			for ct, wal := range s.mw {
				confirmed, unconfirmed := wal.Balance()
				height, _ := wal.ChainTip()
				def, err := repo.MainnetCurrencies().Lookup(ct.CurrencyCode())
				if err != nil {
					def, err = repo.TestnetCurrencies().Lookup(ct.CurrencyCode())
					if err != nil {
						s.log.Errorf("unable to find definition (%s): %s", ct.CurrencyCode(), err.Error())
						continue
					}
				}
				u := walletUpdate{
					Height:      height,
					Unconfirmed: unconfirmed.Value.String(),
					Confirmed:   confirmed.Value.String(),
					Currency:    &def,
				}
				ret[def.CurrencyCode().String()] = u
			}
			ser, err := json.MarshalIndent(walletUpdateWrapper{ret}, "", "    ")
			if err != nil {
				s.log.Errorf("unable to marhsal wallet update: %s", err.Error())
				continue
			}
			s.c <- repo.PremarshalledNotifier{ser}
		case <-s.ctx.Done():
			break
		}
	}
}
