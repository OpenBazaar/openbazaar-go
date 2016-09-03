package bitcoin

import (
	"crypto/sha256"
	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	"strings"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/OpenBazaar/spvwallet"
	"encoding/hex"
)

var log = logging.MustGetLogger("transaction-listener")

type TransactionListener struct {
	db        repo.Datastore
	broadcast chan []byte
	params    *chaincfg.Params
}

func NewTransactionListener(db repo.Datastore, broadcast chan []byte, params *chaincfg.Params) *TransactionListener {
	l := &TransactionListener{db, broadcast, params}
	return l
}

func (l *TransactionListener) OnTransactionReceived(cb spvwallet.TransactionCallback) {
	for _, output := range cb.Outputs {
		if output.IsOurs {
			_, addrs, _, _ := txscript.ExtractPkScriptAddrs(output.ScriptPubKey, l.params)
			contract, _, funded, records, err := l.db.Sales().GetByPaymentAddress(addrs[0])
			if err == nil {
				funding := output.Value
				for _, r := range records {
					funding += r.Value
					// If we've already seen this transaction for some reason, just return
					if r.Txid == hex.EncodeToString(cb.Txid) {
						return
					}
				}
				orderId, err := calcOrderId(contract.BuyerOrder)
				if err != nil {
					return
				}
				if !funded {
					requestedAmount := int64(contract.VendorOrderConfirmation.RequestedAmount)
					if funding >= requestedAmount {
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
				record := spvwallet.TransactionRecord{
					Txid: hex.EncodeToString(cb.Txid),
					Index: output.Index,
					Value: output.Value,
				}
				l.db.Purchases().UpdateFunding(orderId, funded, record)
			}
		} else {
			_, addrs, _, _ := txscript.ExtractPkScriptAddrs(output.ScriptPubKey, l.params)
			contract, _, funded, records, err := l.db.Purchases().GetByPaymentAddress(addrs[0])
			if err == nil {
				funding := output.Value
				for _, r := range records {
					funding += r.Value
					// If we've already seen this transaction for some reason, just return
					if r.Txid == hex.EncodeToString(cb.Txid) {
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
						funded = true
						l.db.Purchases().Put(orderId, *contract, pb.OrderState_FUNDED, true)
					}
				}
				record := spvwallet.TransactionRecord{
					Txid: hex.EncodeToString(cb.Txid),
					Index: output.Index,
					Value: output.Value,
				}
				l.db.Purchases().UpdateFunding(orderId, funded, record)
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
				if c-q < 0 {
					q = 0
					orderId, err := calcOrderId(contract.BuyerOrder)
					if err != nil {
						continue
					}
					log.Warning("Order %s purchased more inventory for %s than we have on hand", orderId, path)
					l.broadcast <- []byte(`{"warning": "order ` + orderId + ` exceeded on hand inventory for ` + path + `"`)
				}
				l.db.Inventory().Put(path, c-q)
				log.Debugf("Adjusting inventory for %s to %d\n", path, c-q)
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
