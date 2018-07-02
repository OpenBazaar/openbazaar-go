package factory

import (
	"github.com/OpenBazaar/openbazaar-go/repo"
)

func NewSaleRecord() *repo.SaleRecord {
	contract := NewContract()
	return &repo.SaleRecord{
		Contract: contract,
		OrderID:  "anOrderIDforaSaleRecord",
	}
}
