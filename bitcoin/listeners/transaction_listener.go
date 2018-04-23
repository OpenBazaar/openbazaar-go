package bitcoin

import (
	"encoding/hex"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("transaction-listener")

type TransactionListener struct {
	db        repo.Datastore
	broadcast chan interface{}
	wallet    wallet.Wallet
	*sync.Mutex
}

func NewTransactionListener(db repo.Datastore, broadcast chan interface{}, wallet wallet.Wallet) *TransactionListener {
	l := &TransactionListener{db, broadcast, wallet, new(sync.Mutex)}
	return l
}

func (l *TransactionListener) OnTransactionReceived(cb wallet.TransactionCallback) {
	l.Lock()
	defer l.Unlock()
	for _, output := range cb.Outputs {
		addr, err := l.wallet.ScriptToAddress(output.ScriptPubKey)
		if err != nil {
			continue
		}
		contract, state, funded, records, err := l.db.Sales().GetByPaymentAddress(addr)
		if err == nil && state != pb.OrderState_PROCESSING_ERROR {
			l.processSalePayment(cb.Txid, output, contract, state, funded, records)
			continue
		}
		contract, state, funded, records, err = l.db.Purchases().GetByPaymentAddress(addr)
		if err == nil {
			l.processPurchasePayment(cb.Txid, output, contract, state, funded, records)
			continue
		}
	}
	for _, input := range cb.Inputs {
		chainHash, err := chainhash.NewHash(cb.Txid)
		if err != nil {
			continue
		}
		addr, err := l.wallet.ScriptToAddress(input.LinkedScriptPubKey)
		if err != nil {
			continue
		}
		isForSale := true
		contract, state, funded, records, err := l.db.Sales().GetByPaymentAddress(addr)
		if err != nil {
			contract, state, funded, records, err = l.db.Purchases().GetByPaymentAddress(addr)
			if err != nil {
				continue
			}
			isForSale = false
		}
		if isForSale && contract.BuyerOrder.Payment.Method == pb.Order_Payment_ADDRESS_REQUEST {
			continue
		}

		orderId, err := calcOrderId(contract.BuyerOrder)
		if err != nil {
			continue
		}

		outpointHash, err := chainhash.NewHash(input.OutpointHash)
		if err != nil {
			continue
		}

		fundsReleased := true
		for i, r := range records {
			if r.Txid == outpointHash.String() && r.Index == input.OutpointIndex {
				records[i].Spent = true
			}
			if records[i].Value > 0 && !records[i].Spent {
				fundsReleased = false
			}
		}

		record := &wallet.TransactionRecord{
			Timestamp:    time.Now(),
			Txid:         chainHash.String(),
			Index:        input.OutpointIndex,
			Value:        -input.Value,
			ScriptPubKey: hex.EncodeToString(input.LinkedScriptPubKey),
		}
		records = append(records, record)
		if isForSale {
			l.db.Sales().UpdateFunding(orderId, funded, records)
			// This is a dispute payout. We should set the order state.
			if state == pb.OrderState_DECIDED && len(records) > 0 && fundsReleased {
				if contract.DisputeAcceptance == nil && contract != nil && contract.BuyerOrder != nil && contract.BuyerOrder.BuyerID != nil {
					accept := new(pb.DisputeAcceptance)
					ts, _ := ptypes.TimestampProto(time.Now())
					accept.Timestamp = ts
					accept.ClosedBy = contract.BuyerOrder.BuyerID.PeerID
					contract.DisputeAcceptance = accept
					buyerHandle := contract.BuyerOrder.BuyerID.Handle

					n := notifications.DisputeAcceptedNotification{
						notifications.NewID(),
						"disputeAccepted",
						orderId,
						notifications.Thumbnail{contract.VendorListings[0].Item.Images[0].Tiny, contract.VendorListings[0].Item.Images[0].Small},
						accept.ClosedBy,
						buyerHandle,
						accept.ClosedBy,
					}

					l.broadcast <- n
					l.db.Notifications().Put(n.ID, n, n.Type, time.Now())
				}
				l.db.Sales().Put(orderId, *contract, pb.OrderState_RESOLVED, false)
			}
		} else {
			l.db.Purchases().UpdateFunding(orderId, funded, records)
			if state == pb.OrderState_DECIDED && len(records) > 0 && fundsReleased {
				if contract.DisputeAcceptance == nil && contract != nil && len(contract.VendorListings) > 0 && contract.VendorListings[0].VendorID != nil {
					accept := new(pb.DisputeAcceptance)
					ts, _ := ptypes.TimestampProto(time.Now())
					accept.Timestamp = ts
					accept.ClosedBy = contract.VendorListings[0].VendorID.PeerID
					contract.DisputeAcceptance = accept
					vendorHandle := contract.VendorListings[0].VendorID.Handle
					var buyer string
					if contract.BuyerOrder != nil && contract.BuyerOrder.BuyerID != nil {
						buyer = contract.BuyerOrder.BuyerID.PeerID
					}

					n := notifications.DisputeAcceptedNotification{
						notifications.NewID(),
						"disputeAccepted",
						orderId,
						notifications.Thumbnail{contract.VendorListings[0].Item.Images[0].Tiny, contract.VendorListings[0].Item.Images[0].Small},
						accept.ClosedBy,
						vendorHandle,
						buyer,
					}

					l.broadcast <- n
					l.db.Notifications().Put(n.ID, n, n.Type, time.Now())
				}
				l.db.Purchases().Put(orderId, *contract, pb.OrderState_RESOLVED, false)
			}
		}
	}

}

func (l *TransactionListener) processSalePayment(txid []byte, output wallet.TransactionOutput, contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord) {
	chainHash, err := chainhash.NewHash(txid)
	if err != nil {
		return
	}
	funding := output.Value
	for _, r := range records {
		funding += r.Value
		// If we have already seen this transaction for some reason, just return
		if r.Txid == chainHash.String() {
			return
		}
	}
	orderId, err := calcOrderId(contract.BuyerOrder)
	if err != nil {
		return
	}
	if !funded {
		requestedAmount := int64(contract.BuyerOrder.Payment.Amount)
		if funding >= requestedAmount {
			log.Debugf("Received payment for order %s", orderId)
			funded = true

			if state == pb.OrderState_AWAITING_PAYMENT && contract.VendorOrderConfirmation != nil { // Confirmed orders go to AWAITING_FULFILLMENT
				l.db.Sales().Put(orderId, *contract, pb.OrderState_AWAITING_FULFILLMENT, false)
			} else if state == pb.OrderState_AWAITING_PAYMENT && contract.VendorOrderConfirmation == nil { // Unconfirmed orders go into PENDING
				l.db.Sales().Put(orderId, *contract, pb.OrderState_PENDING, false)
			}
			l.adjustInventory(contract)

			n := notifications.OrderNotification{
				notifications.NewID(),
				"order",
				contract.VendorListings[0].Item.Title,
				contract.BuyerOrder.BuyerID.PeerID,
				contract.BuyerOrder.BuyerID.Handle,
				notifications.Thumbnail{contract.VendorListings[0].Item.Images[0].Tiny, contract.VendorListings[0].Item.Images[0].Small},
				orderId,
				contract.VendorListings[0].Slug,
			}

			l.broadcast <- n
			l.db.Notifications().Put(n.ID, n, n.Type, time.Now())
		}
	}

	record := &wallet.TransactionRecord{
		Timestamp:    time.Now(),
		Txid:         chainHash.String(),
		Index:        output.Index,
		Value:        output.Value,
		ScriptPubKey: hex.EncodeToString(output.ScriptPubKey),
	}
	records = append(records, record)
	l.db.Sales().UpdateFunding(orderId, funded, records)

	// Save tx metadata
	var thumbnail string
	var title string
	if contract.VendorListings[0].Item != nil && len(contract.VendorListings[0].Item.Images) > 0 {
		thumbnail = contract.VendorListings[0].Item.Images[0].Tiny
		title = contract.VendorListings[0].Item.Title
	}
	bumpable := false
	if contract.BuyerOrder.Payment.Method != pb.Order_Payment_MODERATED {
		bumpable = true
	}
	l.db.TxMetadata().Put(repo.Metadata{chainHash.String(), "", title, orderId, thumbnail, bumpable})
}

func (l *TransactionListener) processPurchasePayment(txid []byte, output wallet.TransactionOutput, contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord) {
	chainHash, err := chainhash.NewHash(txid)
	if err != nil {
		return
	}
	funding := output.Value
	for _, r := range records {
		funding += r.Value
		// If we have already seen this transaction for some reason, just return
		if r.Txid == chainHash.String() {
			return
		}
	}
	orderId, err := calcOrderId(contract.BuyerOrder)
	if err != nil {
		return
	}
	if !funded {
		requestedAmount := int64(contract.BuyerOrder.Payment.Amount)
		if funding >= requestedAmount {
			log.Debugf("Payment for purchase %s detected", orderId)
			funded = true
			if state == pb.OrderState_AWAITING_PAYMENT && contract.VendorOrderConfirmation != nil { // Confirmed orders go to AWAITING_FULFILLMENT
				l.db.Purchases().Put(orderId, *contract, pb.OrderState_AWAITING_FULFILLMENT, false)
			} else if state == pb.OrderState_AWAITING_PAYMENT && contract.VendorOrderConfirmation == nil { // Unconfirmed go into PENDING
				l.db.Purchases().Put(orderId, *contract, pb.OrderState_PENDING, false)
			}
		}
		n := notifications.PaymentNotification{
			notifications.NewID(),
			"payment",
			orderId,
			uint64(funding),
		}
		l.broadcast <- n
		l.db.Notifications().Put(n.ID, n, n.Type, time.Now())
	}

	record := &wallet.TransactionRecord{
		Txid:         chainHash.String(),
		Index:        output.Index,
		Value:        output.Value,
		ScriptPubKey: hex.EncodeToString(output.ScriptPubKey),
		Timestamp:    time.Now(),
	}
	records = append(records, record)
	l.db.Purchases().UpdateFunding(orderId, funded, records)
}

func (l *TransactionListener) adjustInventory(contract *pb.RicardianContract) {
	inventoryUpdated := false
	for _, item := range contract.BuyerOrder.Items {
		listing, err := core.ParseContractForListing(item.ListingHash, contract)
		if err != nil {
			continue
		}
		variant, err := core.GetSelectedSku(listing, item.Options)
		if err != nil {
			continue
		}
		c, err := l.db.Inventory().GetSpecific(listing.Slug, variant)
		if err != nil {
			continue
		}
		q := int(item.Quantity)
		newCount := c - q
		if c < 0 {
			newCount = -1
		} else if newCount < 0 {
			newCount = 0
		}
		if (c == 0) || (c > 0 && c-q < 0) {
			orderId, err := calcOrderId(contract.BuyerOrder)
			if err != nil {
				continue
			}
			log.Warningf("Order %s purchased more inventory for %s than we have on hand", orderId, listing.Slug)
			l.broadcast <- []byte(`{"warning": "order ` + orderId + ` exceeded on hand inventory for ` + listing.Slug + `"`)
		}
		l.db.Inventory().Put(listing.Slug, variant, newCount)
		inventoryUpdated = true
		if newCount >= 0 {
			log.Debugf("Adjusting inventory for %s:%d to %d\n", listing.Slug, variant, newCount)
		}
	}

	if inventoryUpdated && core.Node != nil {
		core.Node.PublishInventory()
	}
}

func calcOrderId(order *pb.Order) (string, error) {
	ser, err := proto.Marshal(order)
	if err != nil {
		return "", err
	}
	id, err := core.EncodeMultihash(ser)
	if err != nil {
		return "", err
	}
	return id.B58String(), nil
}
