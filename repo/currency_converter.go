package repo

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

var (
	// ErrPriceCalculationRequiresExchangeRates - exchange rates dependency err
	ErrPriceCalculationRequiresExchangeRates = errors.New("can't calculate price with exchange rates disabled")
)

type rater interface {
	GetExchangeRate(string) (float64, error)
}

type equivRater struct{}

func (equivRater) GetExchangeRate(_ string) (float64, error) { return 1.0, nil }

// NewEquivalentConverter returns a currency converter where all rates are
// always returns as 1.0 making all currencies equivalent to one another
func NewEquivalentConverter() *CurrencyConverter {
	return &CurrencyConverter{reserveCode: "EQL", reserveRater: equivRater{}}
}

// CurrencyConverter is suitable for converting a currency from one CurrencyDefinition
// to another, accounting for their differing divisibility as well as their differing
// exchange rate as provided by the rater. The rater can represent all rates for the
// reserve currency code provided.
type CurrencyConverter struct {
	reserveCode  string
	reserveRater rater
}

// NewCurrencyConverter returns a valid CurrencyConverter. The rater is verified by
// ensuring the rate of its reserveCode is 1.0 (ensuring it is equivalent to itself).
func NewCurrencyConverter(reserveCode string, rater rater) (*CurrencyConverter, error) {
	var cc = &CurrencyConverter{reserveCode: reserveCode, reserveRater: rater}
	if rate, err := cc.getExchangeRate(cc.reserveCode); err != nil {
		return nil, fmt.Errorf("unable to get reserve rate (%s): %s", cc.reserveCode, err.Error())
	} else if rate != 1.0 {
		return nil, fmt.Errorf("reserve exchange rate was not 1.0 (%f)", rate)
	}
	return cc, nil
}

func (c CurrencyConverter) getExchangeRate(code string) (float64, error) {
	// TODO: remove hack once ExchangeRates can be made aware of testnet currencies
	r, err := c.reserveRater.GetExchangeRate(strings.TrimPrefix(code, "T"))
	if err != nil {
		return 0.0, fmt.Errorf("get rate for (%s): %s", code, err.Error())
	}
	if r <= 0 {
		return 0.0, fmt.Errorf("rate (%f) for (%s) must be greater than zero", r, code)
	}
	return r, nil
}

func (c *CurrencyConverter) getReserveToCurrencyRate(destinationCurrencyCode string) (*big.Float, error) {
	reserveIntoOriginRate, err := c.getExchangeRate(destinationCurrencyCode)
	if err != nil {
		return big.NewFloat(0), err
	}

	// Convert to big.Float
	return new(big.Float).SetFloat64(reserveIntoOriginRate), nil
}

func (c *CurrencyConverter) getConversionRate(originCurrencyCode string, destinationCurrencyCode string) (*big.Float, error) {
	originRate, err := c.getReserveToCurrencyRate(originCurrencyCode)
	if err != nil {
		return big.NewFloat(0), err
	}
	paymentRate, err := c.getReserveToCurrencyRate(destinationCurrencyCode)
	if err != nil {
		return big.NewFloat(0), err
	}
	return new(big.Float).Quo(paymentRate, originRate), nil
}

func (c *CurrencyConverter) getDivisibilityRate(origin, destination CurrencyDefinition) (*big.Float, error) {
	if err := origin.Valid(); err != nil {
		return nil, fmt.Errorf("invalid origin currency: %s", err.Error())
	}
	if err := destination.Valid(); err != nil {
		return nil, fmt.Errorf("invalid destination currency: %s", err.Error())
	}
	originDiv := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), new(big.Int).SetUint64(uint64(origin.Divisibility)), nil))
	destDiv := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), new(big.Int).SetUint64(uint64(destination.Divisibility)), nil))
	return new(big.Float).Quo(destDiv, originDiv), nil
}

// GetFinalPrice returns the resulting CurrencyValue after converting the amount
// and divisibility of the originAmount to the target currency
func (c CurrencyConverter) GetFinalPrice(originAmount *CurrencyValue, resultCurrency CurrencyDefinition) (*CurrencyValue, big.Accuracy, error) {
	originalAmount := new(big.Float).SetInt(originAmount.Amount)

	// get conversion ratio into/out-of reserve currency
	convRate, err := c.getConversionRate(originAmount.Currency.Code.String(), resultCurrency.Code.String())
	if err != nil {
		return nil, 0, err
	}

	// get divisibility ratio between currencies
	divRate, err := c.getDivisibilityRate(originAmount.Currency, resultCurrency)
	if err != nil {
		return nil, 0, err
	}

	// apply the conversion and divisibility ratios to the amount
	finalFloat := new(big.Float).Mul(convRate, originalAmount)
	finalFloat = finalFloat.Mul(finalFloat, divRate)
	finalPrice, acc := finalFloat.Int(nil)

	// round decimal away from zero
	if acc == big.Above {
		finalPrice.Add(finalPrice, big.NewInt(-1))
	}
	if acc == big.Below {
		finalPrice.Add(finalPrice, big.NewInt(1))
	}

	// return -acc because we have rounded to the other side of
	// decimal (0.5 -> 1 which is Above instead of normal behavior
	// of 0.5 -> 0 which is Below)
	return NewCurrencyValueFromBigInt(finalPrice, resultCurrency), -acc, nil
}
