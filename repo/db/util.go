package db

import "github.com/OpenBazaar/openbazaar-go/pb"

func PaymentCoinForContract(contract *pb.RicardianContract) string {
	if contract.BuyerOrder.Payment.AmountCurrency != nil &&
		contract.BuyerOrder.Payment.AmountCurrency.Code != "" {
		return contract.BuyerOrder.Payment.AmountCurrency.Code
	}
	if contract.BuyerOrder.Payment.Coin != "" {
		return contract.BuyerOrder.Payment.Coin
	}
	if len(contract.VendorListings[0].Metadata.AcceptedCurrencies) > 0 {
		return contract.VendorListings[0].Metadata.AcceptedCurrencies[0]
	}
	return ""
}

func CoinTypeForContract(contract *pb.RicardianContract) string {
	if len(contract.VendorListings) > 0 {
		if contract.VendorListings[0].Metadata.CryptoCurrencyCode != "" {
			return contract.VendorListings[0].Metadata.CryptoCurrencyCode
		}
	}
	return ""
}
