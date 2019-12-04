package bitcoin

import (
	"math"
	"math/big"
	"sync"
	"time"

	"github.com/OpenBazaar/multiwallet"
	"github.com/OpenBazaar/wallet-interface"
	btc "github.com/btcsuite/btcutil"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/op/go-logging"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/util"
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

func (l *TransactionListener) OnTransactionReceived(cb wallet.TransactionCallback) {
	log.Info("Transaction received", cb.Txid, cb.Height)

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
			continue
		}
		contract, state, funded, records, err = l.getOrderDetails(output.OrderID, output.Address, false)
		if err == nil {
			l.processPurchasePayment(cb.Txid, output, contract, state, funded, records)
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

		fundsReleased := true
		for i, r := range records {
			if util.AreAddressesEqual(input.LinkedAddress.String(), r.Address) {
				records[i].Spent = true
			}
			if records[i].Value.Cmp(big.NewInt(0)) > 0 && !records[i].Spent {
				fundsReleased = false
			}
		}
		val := new(big.Int).Mul(&input.Value, big.NewInt(-1))
		record := &wallet.TransactionRecord{
			Timestamp: time.Now(),
			Txid:      cb.Txid,
			Index:     input.OutpointIndex,
			Value:     *val,
			Address:   input.LinkedAddress.String(),
		}
		records = append(records, record)
		if isForSale {
			err = l.db.Sales().UpdateFunding(orderId, funded, records)
			if err != nil {
				log.Errorf("update funding for sale (%s): %s", orderId, err)
			}
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
						ID:               repo.NewNotificationID(),
						Type:             "disputeAccepted",
						OrderId:          orderId,
						Thumbnail:        repo.Thumbnail{Tiny: contract.VendorListings[0].Item.Images[0].Tiny, Small: contract.VendorListings[0].Item.Images[0].Small},
						OherPartyID:      accept.ClosedBy,
						OtherPartyHandle: buyerHandle,
						Buyer:            accept.ClosedBy,
					}

					l.broadcast <- n
					err = l.db.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
					if err != nil {
						log.Errorf("persist dispute acceptance notification for order (%s): %s", orderId, err)
					}
				}
				if err := l.db.Sales().Put(orderId, *contract, pb.OrderState_RESOLVED, false); err != nil {
					log.Errorf("failed updating order (%s) to RESOLVED: %s", orderId, err.Error())
				}
			}
		} else {
			err = l.db.Purchases().UpdateFunding(orderId, funded, records)
			if err != nil {
				log.Errorf("update funding for purchase (%s): %s", orderId, err)
			}
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
						ID:               repo.NewNotificationID(),
						Type:             "disputeAccepted",
						OrderId:          orderId,
						Thumbnail:        repo.Thumbnail{Tiny: contract.VendorListings[0].Item.Images[0].Tiny, Small: contract.VendorListings[0].Item.Images[0].Small},
						OherPartyID:      accept.ClosedBy,
						OtherPartyHandle: vendorHandle,
						Buyer:            buyer,
					}

					l.broadcast <- n
					err = l.db.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
					if err != nil {
						log.Errorf("persist dispute acceptance notification for order (%s): %s", orderId, err)
					}
				}
				if err := l.db.Purchases().Put(orderId, *contract, pb.OrderState_RESOLVED, false); err != nil {
					log.Errorf("failed updating order (%s) to RESOLVED: %s", orderId, err.Error())
				}
			}
		}
	}
}

func (l *TransactionListener) processSalePayment(txid string, output wallet.TransactionOutput, contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord) {
	funding := output.Value
	for _, r := range records {
		funding = *new(big.Int).Add(&funding, &r.Value)
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
		currencyValue, err := repo.NewCurrencyValueWithLookup(contract.BuyerOrder.Payment.BigAmount, contract.BuyerOrder.Payment.AmountCurrency.Code)
		if err != nil {
			log.Errorf("Failed parsing CurrencyValue for (%s, %s): %s",
				contract.BuyerOrder.Payment.BigAmount,
				contract.BuyerOrder.Payment.AmountCurrency.Code,
				err.Error(),
			)
			return
		}

		// update divisibility from contract listing
		if contract.VendorListings[0].Metadata.ContractType != pb.Listing_Metadata_CRYPTOCURRENCY {
			customDivisibility := currencyDivisibilityFromContract(l.multiwallet, contract)
			if customDivisibility != currencyValue.Currency.Divisibility {
				currencyValue.Currency.Divisibility = customDivisibility
				if err := currencyValue.Valid(); err != nil {
					log.Errorf("Invalid currency divisibility (%d) found in contract (%s): %s", customDivisibility, orderId, err.Error())
					return
				}
			}
		}

		// TODO: this comparison needs to consider the possibility of different divisibilities
		if funding.Cmp(currencyValue.Amount) >= 0 {
			log.Debugf("Received payment for order %s", orderId)
			funded = true

			if state == pb.OrderState_AWAITING_PAYMENT && contract.VendorOrderConfirmation != nil { // Confirmed orders go to AWAITING_FULFILLMENT
				if err := l.db.Sales().Put(orderId, *contract, pb.OrderState_AWAITING_FULFILLMENT, false); err != nil {
					log.Errorf("failed updating order (%s) to AWAITING_FULFILLMENT: %s", orderId, err.Error())
				}
			} else if state == pb.OrderState_AWAITING_PAYMENT && contract.VendorOrderConfirmation == nil { // Unconfirmed orders go into PENDING
				if err := l.db.Sales().Put(orderId, *contract, pb.OrderState_PENDING, false); err != nil {
					log.Errorf("failed updating order (%s) to PENDING: %s", orderId, err.Error())
				}
			}
			l.adjustInventory(contract)

			n := repo.OrderNotification{
				BuyerHandle:   contract.BuyerOrder.BuyerID.Handle,
				BuyerID:       contract.BuyerOrder.BuyerID.PeerID,
				ID:            repo.NewNotificationID(),
				ListingType:   contract.VendorListings[0].Metadata.ContractType.String(),
				OrderId:       orderId,
				Price:         currencyValue,
				PriceModifier: contract.VendorListings[0].Metadata.PriceModifier,
				Slug:          contract.VendorListings[0].Slug,
				Thumbnail:     repo.Thumbnail{Tiny: contract.VendorListings[0].Item.Images[0].Tiny, Small: contract.VendorListings[0].Item.Images[0].Small},
				Title:         contract.VendorListings[0].Item.Title,
				Type:          "order",
			}

			l.broadcast <- n
			err = l.db.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
			if err != nil {
				log.Error(err)
			}
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
	err = l.db.Sales().UpdateFunding(orderId, funded, records)
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

func currencyDivisibilityFromContract(mw multiwallet.MultiWallet, contract *pb.RicardianContract) uint {
	var currencyDivisibility = contract.VendorListings[0].Item.PriceCurrency.Divisibility
	if currencyDivisibility != 0 {
		return uint(currencyDivisibility)
	}
	wallet, err := mw.WalletForCurrencyCode(contract.BuyerOrder.Payment.AmountCurrency.Code)
	if err == nil {
		return uint(math.Log10(float64(wallet.ExchangeRates().UnitsPerCoin())))
	}
	return core.DefaultCurrencyDivisibility
}

func (l *TransactionListener) processPurchasePayment(txid string, output wallet.TransactionOutput, contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord) {
	funding := output.Value
	for _, r := range records {
		funding = *new(big.Int).Add(&funding, &r.Value)
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
		requestedAmount, _ := new(big.Int).SetString(contract.BuyerOrder.Payment.BigAmount, 10)
		if funding.Cmp(requestedAmount) >= 0 {
			log.Debugf("Payment for purchase %s detected", orderId)
			funded = true
			if state == pb.OrderState_AWAITING_PAYMENT && contract.VendorOrderConfirmation != nil { // Confirmed orders go to AWAITING_FULFILLMENT
				if err := l.db.Purchases().Put(orderId, *contract, pb.OrderState_AWAITING_FULFILLMENT, false); err != nil {
					log.Errorf("failed updating order (%s) to AWAITING_FULFILLMENT: %s", orderId, err.Error())
				}
			} else if state == pb.OrderState_AWAITING_PAYMENT && contract.VendorOrderConfirmation == nil { // Unconfirmed go into PENDING
				if err := l.db.Purchases().Put(orderId, *contract, pb.OrderState_PENDING, false); err != nil {
					log.Errorf("failed updating order (%s) to PENDING: %s", orderId, err.Error())
				}
			}
		}
		def, err := repo.AllCurrencies().Lookup(contract.BuyerOrder.Payment.AmountCurrency.Code)
		if err != nil {
			log.Errorf("Error looking up currency: %s", err)
			return
		}
		cv, err := repo.NewCurrencyValue(funding.String(), def)
		if err != nil {
			log.Errorf("Error creating currency value: %s", err)
			return
		}
		n := repo.PaymentNotification{
			ID:           repo.NewNotificationID(),
			Type:         "payment",
			OrderId:      orderId,
			FundingTotal: cv,
			CoinType:     contract.BuyerOrder.Payment.AmountCurrency.Code,
		}
		l.broadcast <- n
		err = l.db.Notifications().PutRecord(repo.NewNotification(n, time.Now(), false))
		if err != nil {
			log.Error(err)
		}
	}

	record := &wallet.TransactionRecord{
		Txid:      txid,
		Index:     output.Index,
		Value:     output.Value,
		Address:   output.Address.String(),
		Timestamp: time.Now(),
	}
	records = append(records, record)
	err = l.db.Purchases().UpdateFunding(orderId, funded, records)
	if err != nil {
		log.Error(err)
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
		itemQty := core.GetOrderQuantity(listing, item)
		if itemQty.Cmp(big.NewInt(0)) <= 0 || !itemQty.IsInt64() {
			// TODO: https://github.com/OpenBazaar/openbazaar-go/issues/1739
			log.Errorf("unable to update inventory with invalid quantity")
			continue
		}
		newCount := new(big.Int).Sub(c, itemQty)
		if c.Cmp(big.NewInt(0)) < 0 {
			newCount = big.NewInt(-1)
		} else if newCount.Cmp(big.NewInt(0)) < 0 {
			newCount = big.NewInt(0)
		}
		if (c.Cmp(big.NewInt(0)) == 0) || (c.Cmp(big.NewInt(0)) > 0 && new(big.Int).Sub(c, itemQty).Cmp(big.NewInt(0)) < 0) {
			orderId, err := calcOrderId(contract.BuyerOrder)
			if err != nil {
				continue
			}
			log.Warningf("Order %s purchased more inventory for %s than we have on hand", orderId, listing.Slug)
			l.broadcast <- repo.PremarshalledNotifier{Payload: []byte(`{"warning": "order ` + orderId + ` exceeded on hand inventory for ` + listing.Slug + `"`)}
		}
		if err := l.db.Inventory().Put(listing.Slug, variant, newCount); err != nil {
			log.Errorf("failed updating inventory for listing (%s, %d): %s", listing.Slug, variant, err.Error())
		}
		inventoryUpdated = true
		if newCount.Cmp(big.NewInt(0)) >= 0 {
			log.Debugf("Adjusting inventory for %s:%d to %d\n", listing.Slug, variant, newCount)
		}
	}

	if inventoryUpdated && core.Node != nil {
		if err := core.Node.PublishInventory(); err != nil {
			log.Errorf("failed publishing inventory updates: %s", err.Error())
		}
	}
}

func calcOrderId(order *pb.Order) (string, error) {
	ser, err := proto.Marshal(order)
	if err != nil {
		return "", err
	}
	id, err := ipfs.EncodeMultihash(ser)
	if err != nil {
		return "", err
	}
	return id.B58String(), nil
}
