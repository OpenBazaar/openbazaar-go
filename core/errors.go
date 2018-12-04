package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

var (
	// ErrPurchaseUnknownListing - unavailable listing err
	ErrPurchaseUnknownListing = errors.New("order contains a hash of a listing that is not currently for sale")

	// ErrListingDoesNotExist - non-existent listing err
	ErrListingDoesNotExist = errors.New("listing doesn't exist")
	// ErrListingAlreadyExists - duplicate listing err
	ErrListingAlreadyExists = errors.New("listing already exists")
	// ErrListingCoinDivisibilityIncorrect - coin divisibility err
	ErrListingCoinDivisibilityIncorrect = errors.New("incorrect coinDivisibility")
	// ErrPriceCalculationRequiresExchangeRates - exchange rates dependency err
	ErrPriceCalculationRequiresExchangeRates = errors.New("can't calculate price with exchange rates disabled")

	// ErrCryptocurrencyListingCoinTypeRequired - missing coinType err
	ErrCryptocurrencyListingCoinTypeRequired = errors.New("cryptocurrency listings require a coinType")
	// ErrCryptocurrencyPurchasePaymentAddressRequired - missing payment address err
	ErrCryptocurrencyPurchasePaymentAddressRequired = errors.New("paymentAddress required for cryptocurrency items")
	// ErrCryptocurrencyPurchasePaymentAddressTooLong - invalid payment address
	ErrCryptocurrencyPurchasePaymentAddressTooLong = errors.New("paymentAddress required is too long")

	// ErrCryptocurrencySkuQuantityInvalid - invalid sku qty err
	ErrCryptocurrencySkuQuantityInvalid = errors.New("cryptocurrency listing quantity must be a non-negative integer")

	// ErrFulfillIncorrectDeliveryType - incorrect delivery type err
	ErrFulfillIncorrectDeliveryType = errors.New("incorrect delivery type for order")
	// ErrFulfillCryptocurrencyTXIDNotFound - missing txn id err
	ErrFulfillCryptocurrencyTXIDNotFound = errors.New("a transactionID is required to fulfill crypto listings")
	// ErrFulfillCryptocurrencyTXIDTooLong - invalid txn id err
	ErrFulfillCryptocurrencyTXIDTooLong = errors.New("transactionID should be no longer than " + strconv.Itoa(MaxTXIDSize))

	// ErrUnknownWallet is returned when a wallet is not present on the node
	ErrUnknownWallet = errors.New("Unknown wallet type")

	// ErrInvalidSpendAddress is returned when the wallet is unable to decode the string address into a valid destination to send funds to
	ErrInvalidSpendAddress = errors.New("ERROR_INVALID_ADDRESS")

	// ErrInsufficientFunds is returned when the wallet is unable to send the amount specified due to the balance being too low
	ErrInsufficientFunds = errors.New("ERROR_INSUFFICIENT_FUNDS")

	// ErrSpendAmountIsDust is returned when the requested amount to spend out of the wallet would be considered "dust" by the network. This means the value is too low for the network to bother sending the amount and has a high likelihood of not being accepted or being outright rejected.
	ErrSpendAmountIsDust = errors.New("ERROR_DUST_AMOUNT")

	// ErrUnknownOrder is returned when the requested amount to spend is unable to be associated with the appropriate order
	ErrOrderNotFound = errors.New("ERROR_ORDER_NOT_FOUND")
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

// NewErrOutOfInventory - return out of inventory err with available inventory
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
