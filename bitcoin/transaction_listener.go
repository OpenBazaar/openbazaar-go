package bitcoin

import (
	"crypto/sha256"
	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/btcsuite/btcutil"
	"github.com/golang/protobuf/proto"
	mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	"strings"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("transaction-listener")

type TransactionListener struct {
	db        repo.Datastore
	broadcast chan []byte
}

func NewTransactionListener(db repo.Datastore, broadcast chan []byte) *TransactionListener {
	l := &TransactionListener{db, broadcast}
	return l
}

func (l *TransactionListener) OnTransactionReceived(addr btcutil.Address, amount int64, incoming bool) {
	if incoming {
		contract, state, err := l.db.Sales().GetByPaymentAddress(addr)
		if err == nil && int(state) < 2 {
			requestedAmount := contract.VendorOrderConfirmation.RequestedAmount
			if uint64(amount) >= requestedAmount {
				orderId, err := calcOrderId(contract.BuyerOrder)
				if err != nil {
					return
				}
				l.db.Sales().Put(orderId, *contract, pb.OrderState_FUNDED, false)
				l.adjustInventory(contract)

				n := notifications.Serialize(
					notifications.OrderNotification{
						contract.VendorListings[0].Item.Title,
						contract.BuyerOrder.BuyerID.Guid,
						contract.BuyerOrder.BuyerID.BlockchainID,
						contract.VendorListings[0].Item.Images[0].Hash,
						int(contract.BuyerOrder.Timestamp.Seconds),
					})

				l.broadcast <- n
			}
		}
	} else {
		contract, state, err := l.db.Purchases().GetByPaymentAddress(addr)
		if err == nil && int(state) < 2 {
			requestedAmount := contract.BuyerOrder.Payment.Amount
			if uint64(amount) >= requestedAmount {
				orderId, err := calcOrderId(contract.BuyerOrder)
				if err != nil {
					return
				}
				l.db.Sales().Put(orderId, *contract, pb.OrderState_FUNDED, true)
			}
		}
	}
}

func (l *TransactionListener) adjustInventory(contract *pb.RicardianContract) {
	inventory, err := l.db.Inventory().GetAll()
	if err != nil {
		return
	}
	for _, item := range contract.BuyerOrder.Items {
		var variants []string
		for _, option := range item.Options {
			variants = append(variants, option.Value)
		}
		for path, c := range inventory {
			contains := true
			vi:
			for i := 0; i < len(variants); i++ {
				if !strings.Contains(path, variants[i]) {
					contains = false
					break vi
				}
			}
			if contains && c > 0 {
				q := int(item.Quantity)
				if c - q < 0 {
					q = 0
					orderId, err := calcOrderId(contract.BuyerOrder)
					if err != nil {
						continue
					}
					log.Warning("Order %s purchased more inventory for %s than we have on hand", orderId, path)
					l.broadcast <- []byte(`{"warning": "order ` + orderId + ` exceeded on hand inventory for ` + path +`"`)
				}
				l.db.Inventory().Put(path, c - q)
				log.Debugf("Adjusting inventory for %s to %d\n", path, c - q)
			}
		}
	}
}

func calcOrderId(order *pb.Order) (string, error) {
	ser, err := proto.Marshal(order)
	if err != nil {
		return "", err
	}
	orderBytes := sha256.Sum256(ser)
	encoded, err := mh.Encode(orderBytes[:], mh.SHA2_256)
	if err != nil {
		return "", err
	}
	multihash, err := mh.Cast(encoded)
	if err != nil {
		return "", err
	}
	return multihash.B58String(), nil
}
