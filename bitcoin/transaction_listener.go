package bitcoin

import (
	"crypto/sha256"
	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/btcsuite/btcutil"
	"github.com/golang/protobuf/proto"
	mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
)

type TransactionListener struct {
	db        repo.Datastore
	broadcast chan []byte
}

func NewTransactionListener(db repo.Datastore, broadcast chan []byte) *TransactionListener {
	l := &TransactionListener{db, broadcast}
	return l
}

func (l *TransactionListener) OnTransactionReceived(addr btcutil.Address, amount int64) {
	contract, err := l.db.Sales().GetByPaymentAddress(addr)
	if err == nil {
		requestedAmount := contract.VendorOrderConfirmation.RequestedAmount
		if uint64(amount) >= requestedAmount {
			orderId, err := calcOrderId(contract.BuyerOrder)
			if err != nil {
				return
			}
			l.db.Sales().Put(orderId, *contract, pb.OrderState_FUNDED, false)

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
