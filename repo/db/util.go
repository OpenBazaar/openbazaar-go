package db

import "github.com/OpenBazaar/openbazaar-go/pb"

func PaymentCoinForContract(contract *pb.RicardianContract) string {
	paymentCoin := contract.BuyerOrder.Payment.AmountValue.Currency.Code
	if paymentCoin != "" {
		return paymentCoin
	}

	if len(contract.VendorListings[0].Metadata.AcceptedCurrencies) > 0 {
		paymentCoin = contract.VendorListings[0].Metadata.AcceptedCurrencies[0]
	}

	return paymentCoin
}

func CoinTypeForContract(contract *pb.RicardianContract) string {
	coinType := ""

	if len(contract.VendorListings) > 0 && contract.VendorListings[0].Metadata.PricingCurrencyDefn != nil {
		coinType = contract.VendorListings[0].Metadata.PricingCurrencyDefn.Code
	} else if contract.BuyerOrder.Payment.AmountValue != nil {
		coinType = contract.BuyerOrder.Payment.AmountValue.Currency.Code
	}

	return coinType
}
