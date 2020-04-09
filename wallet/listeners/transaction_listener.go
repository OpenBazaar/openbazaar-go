package bitcoin

import (
	"sync"
	"time"

	"github.com/OpenBazaar/multiwallet"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
	btc "github.com/btcsuite/btcutil"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("transaction-listener")

type TransactionListener struct {
	broadcast   chan repo.Notifier
	db          repo.Datastore
	multiwallet multiwallet.MultiWallet
	*sync.Mutex
}

func NewTransactionListener(mw multiwallet.MultiWallet, db repo.Datastore, broadcast chan repo.Notifier) *TransactionListener {
	return &TransactionListener{broadcast, db, mw, new(sync.Mutex)}
}

func (l *TransactionListener) getOrderDetails(orderID string, address btc.Address, isSales bool) (*pb.RicardianContract, pb.OrderState, bool, []*wallet.TransactionRecord, error) {
	var contract *pb.RicardianContract
	var state pb.OrderState
	var funded bool
	var records []*wallet.TransactionRecord
	var err error
	if isSales {
		if orderID != "" {
			contract, state, funded, records, _, _, err = l.db.Sales().GetByOrderId(orderID)
		} else {
			contract, state, funded, records, err = l.db.Sales().GetByPaymentAddress(address)
		}
	} else {
		if orderID != "" {
			contract, state, funded, records, _, _, err = l.db.Purchases().GetByOrderId(orderID)
		} else {
			contract, state, funded, records, err = l.db.Purchases().GetByPaymentAddress(address)
		}
	}

	return contract, state, funded, records, err
}

// cleanupOrderState - scan each order to ensure the state in the db matches the state of the contract stored
func (l *TransactionListener) cleanupOrderState(isSale bool, txid string, output wallet.TransactionOutput, contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord)  {

	orderId, err := calcOrderId(contract.BuyerOrder)
	if err != nil {
		return
	}
	log.Debugf("Cleaning up order state for: #%s\n", orderId)

	if contract.DisputeResolution != nil && state != pb.OrderState_RESOLVED {
		log.Infof("Out of sync order. Found %s and should be %s\n", state, pb.OrderState_RESOLVED)
		if isSale {
			l.db.Sales().Put(orderId, *contract, pb.OrderState_RESOLVED, false)
		} else {
			l.db.Purchases().Put(orderId, *contract, pb.OrderState_RESOLVED, false)
		}

	}
}

func (l *TransactionListener) OnTransactionReceived(cb wallet.TransactionCallback) {
	l.Lock()
	defer l.Unlock()
	for _, output := range cb.Outputs {
		if output.Address == nil {
			continue
		}
		var contract *pb.RicardianContract
		var state pb.OrderState
		var funded bool
		var records []*wallet.TransactionRecord
		var err error

		contract, state, funded, records, err = l.getOrderDetails(output.OrderID, output.Address, true)

		//contract, state, funded, records, err := l.db.Sales().GetByPaymentAddress(output.Address)
		if err == nil && state != pb.OrderState_PROCESSING_ERROR {
			l.processSalePayment(cb.Txid, output, contract, state, funded, records)
			l.cleanupOrderState(true, cb.Txid, output, contract, state, funded, records)
			continue
		}
		contract, state, funded, records, err = l.getOrderDetails(output.OrderID, output.Address, false)
		if err == nil {
			l.processPurchasePayment(cb.Txid, output, contract, state, funded, records)
			l.cleanupOrderState(false, cb.Txid, output, contract, state, funded, records)
			continue
		}
	}
	for _, input := range cb.Inputs {
		if input.LinkedAddress == nil {
			continue
		}
		isForSale := true
		contract, state, funded, records, err := l.getOrderDetails(input.OrderID, input.LinkedAddress, true)
		if err != nil {
			contract, state, funded, records, err = l.getOrderDetails(input.OrderID, input.LinkedAddress, false)
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

		var (
			fundsReleased = true
			unseenTx      = true
		)
		for i, r := range records {
			if input.LinkedAddress.String() == r.Address {
				records[i].Spent = true
			}
			if records[i].Value > 0 && !records[i].Spent {
				fundsReleased = false
			}
			if r.Txid == cb.Txid {
				unseenTx = false
			}
		}

		if unseenTx {
			record := &wallet.TransactionRecord{
				Timestamp: time.Now(),
				Txid:      cb.Txid,
				Index:     input.OutpointIndex,
				Value:     -input.Value,
				Address:   input.LinkedAddress.String(),
			}
			records = append(records, record)
		}
		records = removeDuplicateRecords(records)
		if isForSale {
			l.db.Sales().UpdateFunding(orderId, funded, records)
			// This is a dispute payout. We should set the order state.
			if len(records) > 0 && fundsReleased && contract != nil && contract.Dispute != nil {
				if contract.DisputeAcceptance == nil && contract.BuyerOrder != nil && contract.BuyerOrder.BuyerID != nil {
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
				if state == pb.OrderState_DECIDED {
					if err := l.db.Sales().Put(orderId, *contract, pb.OrderState_RESOLVED, false); err != nil {
						log.Errorf("failed updating order (%s) to RESOLVED: %s", orderId, err.Error())
					}
				} else {
					if err := l.db.Sales().Put(orderId, *contract, state, false); err != nil {
						log.Errorf("failed updating order (%s) with DisputeAcceptance: %s", orderId, err.Error())
					}
				}
			}
		} else {
			err = l.db.Purchases().UpdateFunding(orderId, funded, records)
			if err != nil {
				log.Errorf("update funding for purchase (%s): %s", orderId, err)
			}
			if len(records) > 0 && fundsReleased && contract != nil && contract.Dispute != nil {
				if contract.DisputeAcceptance == nil && len(contract.VendorListings) > 0 && contract.VendorListings[0].VendorID != nil {
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
				if state == pb.OrderState_DECIDED {
					if err := l.db.Purchases().Put(orderId, *contract, pb.OrderState_RESOLVED, false); err != nil {
						log.Errorf("failed updating order (%s) to RESOLVED: %s", orderId, err.Error())
					}
				} else {
					if err := l.db.Purchases().Put(orderId, *contract, state, false); err != nil {
						log.Errorf("failed updating order (%s) with DisputeAcceptance: %s", orderId, err.Error())
					}
				}
			}
		}
	}
}

func (l *TransactionListener) processSalePayment(txid string, output wallet.TransactionOutput, contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord) {
	var (
		funding  = output.Value
		unseenTx = true
	)
	for _, r := range records {
		// If we have already seen this transaction for some reason, just return
		if r.Txid != txid {
			funding += r.Value
		} else {
			unseenTx = false
		}
	}
	orderId, err := calcOrderId(contract.BuyerOrder)
	if err != nil {
		return
	}
	if !funded || (funded && state == pb.OrderState_AWAITING_PAYMENT) {
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
					CoinDivisibility: currencyDivisibilityFromContract(l.multiwallet, contract),
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

	if unseenTx {
		record := &wallet.TransactionRecord{
			Timestamp: time.Now(),
			Txid:      txid,
			Index:     output.Index,
			Value:     output.Value,
			Address:   output.Address.String(),
		}
		records = removeDuplicateRecords(append(records, record))
		l.db.Sales().UpdateFunding(orderId, funded, records)
		if err != nil {
			log.Error(err)
		}

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
		if err := l.db.TxMetadata().Put(repo.Metadata{
			Txid:       txid,
			Address:    "",
			Memo:       title,
			OrderId:    orderId,
			Thumbnail:  thumbnail,
			CanBumpFee: bumpable,
		}); err != nil {
			log.Errorf("failed updating tx metadata (%s): %s", txid, err.Error())
		}
	}
}

func currencyDivisibilityFromContract(mw multiwallet.MultiWallet, contract *pb.RicardianContract) uint32 {
	var currencyDivisibility = contract.VendorListings[0].Metadata.CoinDivisibility
	if currencyDivisibility != 0 {
		return currencyDivisibility
	}
	wallet, err := mw.WalletForCurrencyCode(contract.BuyerOrder.Payment.Coin)
	if err == nil {
		return uint32(wallet.ExchangeRates().UnitsPerCoin())
	}
	return core.DefaultCurrencyDivisibility
}

func (l *TransactionListener) processPurchasePayment(txid string, output wallet.TransactionOutput, contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord) {
	var (
		funding  = output.Value
		unseenTx = true
	)
	for _, r := range records {
		// If we have already seen this transaction for some reason, just return
		if r.Txid != txid {
			funding += r.Value
		} else {
			unseenTx = false
		}
	}
	orderId, err := calcOrderId(contract.BuyerOrder)
	if err != nil {
		return
	}
	if !funded || (funded && state == pb.OrderState_AWAITING_PAYMENT) {
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
			contract.BuyerOrder.Payment.Coin,
		}
		l.broadcast <- n
		l.db.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
	}

	if unseenTx {
		record := &wallet.TransactionRecord{
			Txid:      txid,
			Index:     output.Index,
			Value:     output.Value,
			Address:   output.Address.String(),
			Timestamp: time.Now(),
		}
		records = removeDuplicateRecords(append(records, record))
		err = l.db.Purchases().UpdateFunding(orderId, funded, records)
		if err != nil {
			log.Error(err)
		}
	}
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

func removeDuplicateRecords(recs []*wallet.TransactionRecord) []*wallet.TransactionRecord {
	keys := make(map[string]bool)
	list := []*wallet.TransactionRecord{}
	for _, entry := range recs {
		if _, value := keys[entry.Txid]; !value {
			keys[entry.Txid] = true
			list = append(list, entry)
		}
	}
	return list
}
