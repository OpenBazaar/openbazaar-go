package db

import (
	"errors"
	"github.com/OpenBazaar/openbazaar-go/pb"
)

func PaymentCoinForContract(contract *pb.RicardianContract) (string, error) {
	if contract.BuyerOrder.Payment.AmountCurrency != nil &&
		contract.BuyerOrder.Payment.AmountCurrency.Code != "" {
		return contract.BuyerOrder.Payment.AmountCurrency.Code, nil
	}
	if contract.BuyerOrder.Payment.Coin != "" {
		return contract.BuyerOrder.Payment.Coin, nil
	}
	if len(contract.VendorListings[0].Metadata.AcceptedCurrencies) > 0 {
		return contract.VendorListings[0].Metadata.AcceptedCurrencies[0], nil
	}
	return "", errors.New("payment coin not found")
}

func CoinTypeForContract(contract *pb.RicardianContract) string {
	if len(contract.VendorListings) > 0 && contract.VendorListings[0].Metadata.ContractType == pb.Listing_Metadata_CRYPTOCURRENCY {
		return contract.VendorListings[0].Metadata.CryptoCurrencyCode
	}
	return ""
}
