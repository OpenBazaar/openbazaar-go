package db

import "github.com/OpenBazaar/openbazaar-go/pb"

func PaymentCoinForContract(contract *pb.RicardianContract) string {
	paymentCoin := contract.BuyerOrder.Payment.Amount.Currency.Code
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

	if len(contract.VendorListings) > 0 && contract.VendorListings[0].Metadata.PricingCurrency != nil {
		coinType = contract.VendorListings[0].Metadata.PricingCurrency.Code
	} else if contract.BuyerOrder.Payment.Amount != nil {
		coinType = contract.BuyerOrder.Payment.Amount.Currency.Code
	}

	return coinType
}
