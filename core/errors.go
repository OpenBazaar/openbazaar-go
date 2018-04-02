package core

import (
	"errors"
	"fmt"
)

var (
	ErrPurchaseUnknownListing = errors.New("Order contains a hash of a listing that is not currently for sale")

	ErrListingDoesNotExist              = errors.New("Listing doesn't exist")
	ErrListingAlreadyExists             = errors.New("Listing already exists")
	ErrListingCoinDivisibilityIncorrect = errors.New("Incorrect coinDivisibility")

	ErrCryptocurrencyListingCoinTypeRequired        = errors.New("Cryptocurrency listings require a coinType")
	ErrCryptocurrencyPurchasePaymentAddressRequired = errors.New("paymentAddress required for cryptocurrency items")

	ErrPriceCalculationRequiresExchangeRates = errors.New("Can't calculate price with exchange rates disabled")
)

type ErrCryptocurrencyListingIllegalField string

func (e ErrCryptocurrencyListingIllegalField) Error() string {
	return illegalFieldString("cryptocurrency listing", string(e))
}

type ErrCryptocurrencyPurchaseIllegalField string

func (e ErrCryptocurrencyPurchaseIllegalField) Error() string {
	return illegalFieldString("cryptocurrency purchase", string(e))
}

type ErrMarketPriceListingIllegalField string

func (e ErrMarketPriceListingIllegalField) Error() string {
	return illegalFieldString("market price listing", string(e))
}

func illegalFieldString(objectType string, field string) string {
	return fmt.Sprintf("Illegal %s field: %s", objectType, field)
}
