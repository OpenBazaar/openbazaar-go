package core

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	util "gx/ipfs/QmNohiVssaPw3KVLZik59DBVGTSm2dGvYT9eoXt5DQ36Yz/go-ipfs-util"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/golang/protobuf/ptypes"
	google_protobuf "github.com/golang/protobuf/ptypes/timestamp"
)

// FormatRFC3339PB returns the given `google_protobuf.Timestamp` as a RFC3339
// formatted string
func FormatRFC3339PB(ts google_protobuf.Timestamp) string {
	return util.FormatRFC3339(time.Unix(ts.Seconds, int64(ts.Nanos)).UTC())
}

// BuildTransactionRecords - Used by the GET order API to build transaction records suitable to be included in the order response
func (n *OpenBazaarNode) BuildTransactionRecords(contract *pb.RicardianContract, records []*wallet.TransactionRecord, state pb.OrderState) ([]*pb.TransactionRecord, *pb.TransactionRecord, error) {
	paymentRecords := []*pb.TransactionRecord{}
	payments := make(map[string]*pb.TransactionRecord)
	order, err := repo.ToV5Order(contract.BuyerOrder, n.LookupCurrency)
	if err != nil {
		return nil, nil, err
	}
	wal, err := n.Multiwallet.WalletForCurrencyCode(order.Payment.AmountCurrency.Code)
	if err != nil {
		return paymentRecords, nil, err
	}

	for _, r := range records {
		_, ok := payments[r.Txid]
		if !ok {
			tx := new(pb.TransactionRecord)
			tx.Txid = r.Txid
			tx.BigValue = r.Value.String()
			tx.Currency = order.Payment.AmountCurrency

			ts, err := ptypes.TimestampProto(r.Timestamp)
			if err != nil {
				return paymentRecords, nil, err
			}
			tx.Timestamp = ts

			confirmations, height, err := wal.GetConfirmations(tx.Txid)
			if err != nil {
				return paymentRecords, nil, err
			}
			tx.Height = height
			tx.Confirmations = confirmations
			payments[r.Txid] = tx
		}
	}
	for _, rec := range payments {
		paymentRecords = append(paymentRecords, rec)
	}
	var refundRecord *pb.TransactionRecord
	if contract != nil && (state == pb.OrderState_REFUNDED || state == pb.OrderState_DECLINED || state == pb.OrderState_CANCELED) && order.Payment != nil {
		// For multisig we can use the outgoing from the payment address
		if order.Payment.Method == pb.Order_Payment_MODERATED || state == pb.OrderState_DECLINED || state == pb.OrderState_CANCELED {
			for _, rec := range payments {
				val, _ := new(big.Int).SetString(rec.BigValue, 10)
				if val.Cmp(big.NewInt(0)) < 0 {
					refundRecord = new(pb.TransactionRecord)
					refundRecord.Txid = rec.Txid
					refundRecord.BigValue = rec.BigValue
					refundRecord.Currency = rec.Currency
					refundRecord.Confirmations = rec.Confirmations
					refundRecord.Height = rec.Height
					refundRecord.Timestamp = rec.Timestamp

					if !strings.HasPrefix(refundRecord.BigValue, "-") {
						refundRecord.BigValue = "-" + refundRecord.BigValue
					}
					break
				}
			}
		} else if contract.Refund != nil && contract.Refund.RefundTransaction != nil && contract.Refund.Timestamp != nil {
			refund := repo.ToV5Refund(contract.Refund)
			refundRecord = new(pb.TransactionRecord)
			// Direct we need to use the transaction info in the contract's refund object
			ch, err := chainhash.NewHashFromStr(strings.TrimPrefix(contract.Refund.RefundTransaction.Txid, "0x"))
			if err != nil {
				return paymentRecords, refundRecord, err
			}
			confirmations, height, err := wal.GetConfirmations(ch.String())
			if err != nil {
				return paymentRecords, refundRecord, nil
			}
			refundRecord.Txid = refund.RefundTransaction.Txid
			refundRecord.BigValue = refund.RefundTransaction.BigValue
			refundRecord.Currency = refund.RefundTransaction.ValueCurrency
			refundRecord.Timestamp = refund.Timestamp
			refundRecord.Confirmations = confirmations
			refundRecord.Height = height
		}
	}
	return paymentRecords, refundRecord, nil
}

// LookupCurrency looks up the CurrencyDefinition from available currencies
func (n *OpenBazaarNode) LookupCurrency(currencyCode string) (repo.CurrencyDefinition, error) {
	return repo.AllCurrencies().Lookup(currencyCode)
}

func (n *OpenBazaarNode) ValidateMultiwalletHasPreferredCurrencies(data repo.SettingsData) error {
	if data.PreferredCurrencies == nil {
		return nil
	}
	for _, cc := range *data.PreferredCurrencies {
		_, err := n.Multiwallet.WalletForCurrencyCode(cc)
		if err != nil {
			return fmt.Errorf("preferred coin %s not found in multiwallet", cc)
		}
	}
	return nil
}
