package core

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
)

const (
	RatingMin           = 1
	RatingMax           = 5
	ReviewMaxCharacters = 3000
)

type RatingData struct {
	OrderId         string `json:"orderId"`
	Quality         int    `json:"quality"`
	Description     int    `json:"description"`
	DeliverySpeed   int    `json:"deliverySpeed"`
	CustomerService int    `json:"customerService"`
	Review          string `json:"rewview"`
}

func (n *OpenBazaarNode) CompleteOrder(ratingData *RatingData, contract *pb.RicardianContract, records []*spvwallet.TransactionRecord) error {
	// TODO: build this out
	return nil
}
