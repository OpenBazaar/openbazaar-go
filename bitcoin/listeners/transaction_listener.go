package bitcoin

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	mh "gx/ipfs/QmbZ6Cee2uHjG7hf19qLHppgKDRtaG4CVtMzdmK9VCVqLu/go-multihash"
	"sync"
	"time"
)

var log = logging.MustGetLogger("transaction-listener")

type TransactionListener struct {
	db        repo.Datastore
	broadcast chan interface{}
	wallet    bitcoin.BitcoinWallet
	*sync.Mutex
}

func NewTransactionListener(db repo.Datastore, broadcast chan interface{}, wallet bitcoin.BitcoinWallet) *TransactionListener {
	l := &TransactionListener{db, broadcast, wallet, new(sync.Mutex)}
	return l
}

func (l *TransactionListener) OnTransactionReceived(cb spvwallet.TransactionCallback) {
	l.Lock()
	defer l.Unlock()
	for _, output := range cb.Outputs {
		addr, err := l.wallet.ScriptToAddress(output.ScriptPubKey)
		if err != nil {
			continue
		}
		contract, state, funded, records, err := l.db.Sales().GetByPaymentAddress(addr)
		if err == nil {
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

		orderId, err := calcOrderId(contract.BuyerOrder)
		if err != nil {
			continue
		}

		outpointHash, err := chainhash.NewHash(input.OutpointHash)
		if err != nil {
			continue
		}

		var fundsReleased bool
		for _, r := range records {
			if r.Txid == outpointHash.String() && r.Index == input.OutpointIndex {
				r.Spent = true
				fundsReleased = true
			}
		}

		record := &spvwallet.TransactionRecord{
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
				l.db.Sales().Put(orderId, *contract, pb.OrderState_RESOLVED, false)
			}
		} else {
			l.db.Purchases().UpdateFunding(orderId, funded, records)
			if state == pb.OrderState_DECIDED && len(records) > 0 && fundsReleased {
				l.db.Purchases().Put(orderId, *contract, pb.OrderState_RESOLVED, false)
			}
		}
	}

}

func (l *TransactionListener) processSalePayment(txid []byte, output spvwallet.TransactionOutput, contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*spvwallet.TransactionRecord) {
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
			log.Debugf("Recieved payment for order %s", orderId)
			funded = true

			if state == pb.OrderState_AWAITING_PAYMENT && contract.VendorOrderConfirmation != nil { // Confirmed orders go to AWAITING_FULFILLMENT
				l.db.Sales().Put(orderId, *contract, pb.OrderState_AWAITING_FULFILLMENT, false)
			} else if state == pb.OrderState_AWAITING_PAYMENT && contract.VendorOrderConfirmation == nil { // Unconfirmed orders go into PENDING
				l.db.Sales().Put(orderId, *contract, pb.OrderState_PENDING, false)
			}
			l.adjustInventory(contract)

			n := notifications.OrderNotification{
				contract.VendorListings[0].Item.Title,
				contract.BuyerOrder.BuyerID.PeerID,
				contract.BuyerOrder.BuyerID.BlockchainID,
				contract.VendorListings[0].Item.Images[0].Tiny,
				int(contract.BuyerOrder.Timestamp.Seconds),
				orderId,
			}

			l.broadcast <- n
			l.db.Notifications().Put(notifications.Wrap(n), time.Now())
		}
	}

	record := &spvwallet.TransactionRecord{
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

func (l *TransactionListener) processPurchasePayment(txid []byte, output spvwallet.TransactionOutput, contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*spvwallet.TransactionRecord) {
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
			orderId,
			uint64(funding),
		}
		l.broadcast <- n
		l.db.Notifications().Put(notifications.Wrap(n), time.Now())
	}

	record := &spvwallet.TransactionRecord{
		Txid:         chainHash.String(),
		Index:        output.Index,
		Value:        output.Value,
		ScriptPubKey: hex.EncodeToString(output.ScriptPubKey),
	}
	records = append(records, record)
	l.db.Purchases().UpdateFunding(orderId, funded, records)
}

func (l *TransactionListener) adjustInventory(contract *pb.RicardianContract) {
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
		if c-q < 0 {
			q = 0
			orderId, err := calcOrderId(contract.BuyerOrder)
			if err != nil {
				continue
			}
			log.Warning("Order %s purchased more inventory for %s than we have on hand", orderId, listing.Slug)
			l.broadcast <- []byte(`{"warning": "order ` + orderId + ` exceeded on hand inventory for ` + listing.Slug + `"`)
		}
		l.db.Inventory().Put(listing.Slug, variant, c-q)
		log.Debugf("Adjusting inventory for %s:%d to %d\n", listing.Slug, variant, c-q)
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
