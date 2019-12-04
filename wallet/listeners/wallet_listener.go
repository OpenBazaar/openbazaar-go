package bitcoin

import (
	"math/big"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
)

type WalletListener struct {
	db        repo.Datastore
	broadcast chan repo.Notifier
	coinType  wallet.CoinType
}

func NewWalletListener(db repo.Datastore, broadcast chan repo.Notifier, coinType wallet.CoinType) *WalletListener {
	l := &WalletListener{db, broadcast, coinType}
	return l
}

func (l *WalletListener) OnTransactionReceived(cb wallet.TransactionCallback) {
	if !cb.WatchOnly {
		metadata, err := l.db.TxMetadata().Get(cb.Txid)
		if err != nil {
			log.Debugf("tx metadata not found for id (%s): %s", cb.Txid, err.Error())
		}

		status := "UNCONFIRMED"
		confirmations := 0
		if cb.Height > 0 {
			status = "PENDING"
			confirmations = 1
		}

		txValue, err := repo.NewCurrencyValueWithLookup(cb.Value.String(), l.coinType.CurrencyCode())
		if err != nil {
			log.Errorf("failed parsing currency value (%s %s): %s", cb.Value.String(), l.coinType.CurrencyCode(), err.Error())
			return
		}

		l.broadcast <- repo.IncomingTransaction{
			Txid:          cb.Txid,
			Value:         txValue,
			Address:       metadata.Address,
			Status:        status,
			Memo:          metadata.Memo,
			Timestamp:     cb.Timestamp,
			Confirmations: int32(confirmations),
			OrderId:       metadata.OrderId,
			Thumbnail:     metadata.Thumbnail,
			Height:        cb.Height,
			CanBumpFee:    cb.Value.Cmp(big.NewInt(0)) > 0,
		}
	}
}
