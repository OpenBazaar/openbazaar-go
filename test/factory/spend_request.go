package factory

import (
	"github.com/OpenBazaar/openbazaar-go/core"
)

func NewSpendRequest() *core.SpendRequest {
	return &core.SpendRequest{
		CurrencyCode:           "BTC",
		Address:                "1HYhu8e2wv19LZ2umXoo1pMiwzy2rL32UQ",
		Amount:                 "1234",
		FeeLevel:               "PRIORITY",
		RequireAssociatedOrder: false,
	}
}
