package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/ptypes"
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

// SendProcessingError will encapsulate the failing state in a message to be sent back to pid
// When pid receives the OrderProcessingError, it will analyze the contract and send the messages
// that this node is missing to resynchronize the order
func (n *OpenBazaarNode) SendProcessingError(pid, oid string, attemptedMessage pb.Message_MessageType, latestContract *pb.RicardianContract) error {
	log.Debugf("sending ORDER_PROCESSING_ERROR to peer (%s)", pid)
	var (
		procErrMsg = &pb.OrderProcessingFailure{
			OrderID:              oid,
			AttemptedMessageType: attemptedMessage,
			Contract:             latestContract,
		}
		procErrBytes, mErr = ptypes.MarshalAny(procErrMsg)
		errMsg             = &pb.Message{
			MessageType: pb.Message_ORDER_PROCESSING_FAILURE,
			Payload:     procErrBytes,
		}
		p, pErr = peer.IDB58Decode(pid)
	)
	if mErr != nil {
		log.Errorf("failed marshaling OrderProcessingFailure message for order (%s): %s", oid, mErr)
		return mErr
	}
	if pErr != nil {
		log.Errorf("failed decoding peer ID (%s): %s", pid, pErr)
		return pErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), n.OfflineMessageFailoverTimeout)
	defer cancel()

	return n.Service.SendMessage(ctx, p, errMsg)
}
