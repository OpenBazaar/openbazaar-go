package repo

import (
	"errors"
	"fmt"
)

var (
	// ErrCryptocurrencySkuQuantityInvalid - invalid sku qty err
	ErrCryptocurrencySkuQuantityInvalid = errors.New("cryptocurrency listing quantity must be a non-negative integer")
	// ErrListingCryptoDivisibilityInvalid indicates the given divisibility doesn't match the default
	ErrListingCryptoDivisibilityInvalid = errors.New("invalid cryptocurrency divisibility")
	// ErrListingCryptoCurrencyCodeInvalid indicates the given code isn't valid or known
	ErrListingCryptoCurrencyCodeInvalid = errors.New("invalid cryptocurrency code")
	ErrShippingRegionMustBeSet          = errors.New("shipping region must be set")
	ErrShippingRegionUndefined          = errors.New("undefined shipping region")
	ErrShippingRegionMustNotBeContinent = errors.New("cannot specify continent as shipping region")
	// ErrListingDoesNotExist - non-existent listing err
	ErrListingDoesNotExist = errors.New("listing doesn't exist")
	// ErrListingAlreadyExists - duplicate listing err
	ErrListingAlreadyExists = errors.New("listing already exists")
)

// ErrPriceModifierOutOfRange - customize limits for price modifier
type ErrPriceModifierOutOfRange struct {
	Min float64
	Max float64
}

func (e ErrPriceModifierOutOfRange) Error() string {
	return fmt.Sprintf("priceModifier out of range: [%.2f, %.2f]", e.Min, e.Max)
}

// ErrCryptocurrencyListingIllegalField - invalid field err
type ErrCryptocurrencyListingIllegalField string

func (e ErrCryptocurrencyListingIllegalField) Error() string {
	return illegalFieldString("cryptocurrency listing", string(e))
}

// ErrCryptocurrencyPurchaseIllegalField - invalid purchase field err
type ErrCryptocurrencyPurchaseIllegalField string

func (e ErrCryptocurrencyPurchaseIllegalField) Error() string {
	return illegalFieldString("cryptocurrency purchase", string(e))
}

// ErrMarketPriceListingIllegalField - invalid listing field err
type ErrMarketPriceListingIllegalField string

func (e ErrMarketPriceListingIllegalField) Error() string {
	return illegalFieldString("market price listing", string(e))
}

func illegalFieldString(objectType string, field string) string {
	return fmt.Sprintf("Illegal %s field: %s", objectType, field)
}
