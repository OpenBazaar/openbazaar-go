package factory

import (
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

func NewPurchaseRecord() *repo.PurchaseRecord {
	contract := NewContract()
	return &repo.PurchaseRecord{
		Contract: contract,
		OrderID:  "anOrderIDforaPurchaseRecord",
	}
}

func NewExpiredPurchaseRecord() *repo.PurchaseRecord {
	purchase := NewPurchaseRecord()
	purchase.Timestamp = time.Now().Add(-repo.BuyerDisputeTimeout_totalDuration)
	return purchase
}

func NewExpiredDisputeablePurchaseRecord() *repo.PurchaseRecord {
	purchase := NewExpiredPurchaseRecord()
	purchase.Contract = NewDisputeableContract()
	return purchase
}
