package bitcoin

import (
	"sync"
	"time"

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
	broadcast chan repo.Notifier
	*sync.Mutex
}

func NewTransactionListener(db repo.Datastore, broadcast chan repo.Notifier) *TransactionListener {
	l := &TransactionListener{db, broadcast, new(sync.Mutex)}
	return l
}

func (l *TransactionListener) OnTransactionReceived(cb wallet.TransactionCallback) {
	l.Lock()
	defer l.Unlock()
	for _, output := range cb.Outputs {
		contract, state, funded, records, err := l.db.Sales().GetByPaymentAddress(output.Address)
		if err == nil && state != pb.OrderState_PROCESSING_ERROR {
			l.processSalePayment(cb.Txid, output, contract, state, funded, records)
			continue
		}
		contract, state, funded, records, err = l.db.Purchases().GetByPaymentAddress(output.Address)
		if err == nil {
			l.processPurchasePayment(cb.Txid, output, contract, state, funded, records)
			continue
		}
	}
	for _, input := range cb.Inputs {
		isForSale := true
		contract, state, funded, records, err := l.db.Sales().GetByPaymentAddress(input.LinkedAddress)
		if err != nil {
			contract, state, funded, records, err = l.db.Purchases().GetByPaymentAddress(input.LinkedAddress)
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
			Timestamp: time.Now(),
			Txid:      cb.Txid,
			Index:     input.OutpointIndex,
			Value:     -input.Value,
			Address:   input.LinkedAddress.String(),
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

					n := repo.DisputeAcceptedNotification{
						repo.NewNotificationID(),
						"disputeAccepted",
						orderId,
						repo.Thumbnail{contract.VendorListings[0].Item.Images[0].Tiny, contract.VendorListings[0].Item.Images[0].Small},
						accept.ClosedBy,
						buyerHandle,
						accept.ClosedBy,
					}

					l.broadcast <- n
					l.db.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
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

					n := repo.DisputeAcceptedNotification{
						repo.NewNotificationID(),
						"disputeAccepted",
						orderId,
						repo.Thumbnail{contract.VendorListings[0].Item.Images[0].Tiny, contract.VendorListings[0].Item.Images[0].Small},
						accept.ClosedBy,
						vendorHandle,
						buyer,
					}

					l.broadcast <- n
					l.db.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
				}
				l.db.Purchases().Put(orderId, *contract, pb.OrderState_RESOLVED, false)
			}
		}
	}
}

func (l *TransactionListener) processSalePayment(txid string, output wallet.TransactionOutput, contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord) {
	var funding = output.Value
	for _, r := range records {
		funding += r.Value
		// If we have already seen this transaction for some reason, just return
		if r.Txid == txid {
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

			n := repo.OrderNotification{
				BuyerHandle: contract.BuyerOrder.BuyerID.Handle,
				BuyerID:     contract.BuyerOrder.BuyerID.PeerID,
				ID:          repo.NewNotificationID(),
				ListingType: contract.VendorListings[0].Metadata.ContractType.String(),
				OrderId:     orderId,
				Price: repo.ListingPrice{
					Amount:           contract.BuyerOrder.Payment.Amount,
					CoinDivisibility: currencyDivisibilityFromContract(contract, orderId),
					CurrencyCode:     contract.BuyerOrder.Payment.Coin,
					PriceModifier:    contract.VendorListings[0].Metadata.PriceModifier,
				},
				Slug:      contract.VendorListings[0].Slug,
				Thumbnail: repo.Thumbnail{contract.VendorListings[0].Item.Images[0].Tiny, contract.VendorListings[0].Item.Images[0].Small},
				Title:     contract.VendorListings[0].Item.Title,
				Type:      "order",
			}

			l.broadcast <- n
			l.db.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
		}
	}

	record := &wallet.TransactionRecord{
		Timestamp: time.Now(),
		Txid:      txid,
		Index:     output.Index,
		Value:     output.Value,
		Address:   output.Address.String(),
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
	l.db.TxMetadata().Put(repo.Metadata{txid, "", title, orderId, thumbnail, bumpable})
}

func currencyDivisibilityFromContract(contract *pb.RicardianContract, orderID string) uint32 {
	var currencyDivisibility = contract.VendorListings[0].Metadata.CoinDivisibility
	if currencyDivisibility != 0 {
		return currencyDivisibility
	}

	currency, err := core.CurrencyFromString(contract.BuyerOrder.Payment.Coin)
	if err != nil {
		log.Errorf("processing payment: currency divisibility: unsupported currency %s for order %s", contract.BuyerOrder.Payment.Coin, orderID)
		return core.DefaultCurrencyDivisibility
	}
	return currency.Divisibility()
}

func (l *TransactionListener) processPurchasePayment(txid string, output wallet.TransactionOutput, contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord) {
	funding := output.Value
	for _, r := range records {
		funding += r.Value
		// If we have already seen this transaction for some reason, just return
		if r.Txid == txid {
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
		n := repo.PaymentNotification{
			repo.NewNotificationID(),
			"payment",
			orderId,
			uint64(funding),
		}
		l.broadcast <- n
		l.db.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
	}

	record := &wallet.TransactionRecord{
		Txid:      txid,
		Index:     output.Index,
		Value:     output.Value,
		Address:   output.Address.String(),
		Timestamp: time.Now(),
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
		q := int64(core.GetOrderQuantity(listing, item))
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
			l.broadcast <- repo.PremarshalledNotifier{[]byte(`{"warning": "order ` + orderId + ` exceeded on hand inventory for ` + listing.Slug + `"`)}
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
