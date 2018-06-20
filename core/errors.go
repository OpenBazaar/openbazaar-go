package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

var (
	ErrPurchaseUnknownListing = errors.New("Order contains a hash of a listing that is not currently for sale")

	ErrListingDoesNotExist                   = errors.New("Listing doesn't exist")
	ErrListingAlreadyExists                  = errors.New("Listing already exists")
	ErrListingCoinDivisibilityIncorrect      = errors.New("Incorrect coinDivisibility")
	ErrPriceCalculationRequiresExchangeRates = errors.New("Can't calculate price with exchange rates disabled")

	ErrCryptocurrencyListingCoinTypeRequired        = errors.New("Cryptocurrency listings require a coinType")
	ErrCryptocurrencyPurchasePaymentAddressRequired = errors.New("paymentAddress required for cryptocurrency items")
	ErrCryptocurrencyPurchasePaymentAddressTooLong  = errors.New("paymentAddress required is too long")

	ErrCryptocurrencySkuQuantityInvalid = errors.New("Cryptocurrency listing quantity must be a non-negative integer")

	ErrFulfillIncorrectDeliveryType      = errors.New("Incorrect delivery type for order")
	ErrFulfillCryptocurrencyTXIDNotFound = errors.New("A transactionID is required to fulfill crypto listings")
	ErrFulfillCryptocurrencyTXIDTooLong  = errors.New("transactionID should be no longer than " + strconv.Itoa(MaxTXIDSize))
)

// CodedError is an error that is machine readable
type CodedError struct {
	Reason string `json:"reason,omitempty"`
	Code   string `json:"code,omitempty"`
}

func (err CodedError) Error() string {
	jsonBytes, _ := json.Marshal(&err)
	return string(jsonBytes)
}

// ErrOutOfInventory is a codedError returned from vendor nodes when buyers try
// purchasing too many of an item
type ErrOutOfInventory struct {
	CodedError
	RemainingInventory int64 `json:"remainingInventory"`
}

func NewErrOutOfInventory(inventoryRemaining int64) ErrOutOfInventory {
	return ErrOutOfInventory{
		CodedError: CodedError{
			Reason: "not enough inventory",
			Code:   "ERR_INSUFFICIENT_INVENTORY",
		},
		RemainingInventory: inventoryRemaining,
	}
}

func (err ErrOutOfInventory) Error() string {
	jsonBytes, _ := json.Marshal(&err)
	return string(jsonBytes)
}

type ErrPriceModifierOutOfRange struct {
	Min float64
	Max float64
}

func (e ErrPriceModifierOutOfRange) Error() string {
	return fmt.Sprintf("priceModifier out of range: [%.2f, %.2f]", e.Min, e.Max)
}

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
