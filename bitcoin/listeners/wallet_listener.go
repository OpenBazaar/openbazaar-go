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
	if !cb.WatchOnly && cb.Value > 0 {
		txid := hex.EncodeToString(cb.Txid)
		metadata, _ := l.db.TxMetadata().Get(txid)
		n := notifications.IncomingTransaction{
			Txid:          hex.EncodeToString(cb.Txid),
			Value:         cb.Value,
			Address:       metadata.Address,
			Status:        "UNCONFIRMED",
			Memo:          metadata.Memo,
			Timestamp:     cb.Timestamp,
			Confirmations: 0,
			OrderId:       metadata.OrderId,
			Thumbnail:     metadata.Thumbnail,
			CanBumpFee:    true,
		}
		l.broadcast <- n
	}
}
