package db

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
)

func paymentCoinForContract(contract *pb.RicardianContract) string {
	paymentCoin := contract.BuyerOrder.Payment.Coin
	if paymentCoin != "" {
		return paymentCoin
	}

	if len(contract.VendorListings[0].Metadata.AcceptedCurrencies) > 0 {
		paymentCoin = contract.VendorListings[0].Metadata.AcceptedCurrencies[0]
	}

	return paymentCoin
}

func coinTypeForContract(contract *pb.RicardianContract) string {
	coinType := ""

	if len(contract.VendorListings) > 0 {
		coinType = contract.VendorListings[0].Metadata.CoinType
	}

	return coinType
}
