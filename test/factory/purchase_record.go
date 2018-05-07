package factory

import (
	"github.com/OpenBazaar/openbazaar-go/repo"
)

func NewPurchaseRecord() *repo.PurchaseRecord {
	contract := NewContract()
	return &repo.PurchaseRecord{
		Contract: contract,
		OrderID:  "anOrderIDforaPurchaseRecord",
	}
}
