package bitcoin

import (
	"encoding/hex"
	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/spvwallet"
)

type WalletListener struct {
	db        repo.Datastore
	broadcast chan interface{}
}

func NewWalletListener(db repo.Datastore, broadcast chan interface{}) *WalletListener {
	l := &WalletListener{db, broadcast}
	return l
}

func (l *WalletListener) OnTransactionReceived(cb spvwallet.TransactionCallback) {
	log.Notice(cb.WatchOnly, cb.Value)
	if !cb.WatchOnly && cb.Value > 0 {
		txid := hex.EncodeToString(cb.Txid)
		metadata, _ := l.db.TxMetadata().Get(txid)
		status := "UNCONFIRMED"
		if cb.Height > 0 && cb.Height < 6 {
			status = "PENDING"
		} else if cb.Height >= 6 {
			status = "CONFIRMED"
		}
		n := notifications.IncomingTransaction{
			Txid:          hex.EncodeToString(cb.Txid),
			Value:         cb.Value,
			Address:       metadata.Address,
			Status:        status,
			Memo:          metadata.Memo,
			Timestamp:     cb.Timestamp,
			Confirmations: 0,
			OrderId:       metadata.OrderId,
			Thumbnail:     metadata.Thumbnail,
			Height:        cb.Height,
			CanBumpFee:    true,
		}
		l.broadcast <- n
	}
}
