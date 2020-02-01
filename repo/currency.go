package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

const (
	CurrencyCodeValidMinimumLength = 3
	CurrencyCodeValidMaximumLength = 4
)

var (
	ErrCurrencyValueInsufficientPrecision         = errors.New("unable to accurately represent value as int64")
	ErrCurrencyValueNegativeRate                  = errors.New("conversion rate must be greater than zero")
	ErrCurrencyValueAmountInvalid                 = errors.New("invalid amount")
	ErrCurrencyValueDefinitionInvalid             = errors.New("invalid currency definition")
	ErrCurrencyValueInvalidCmpDifferentCurrencies = errors.New("unable to compare two different currencies")
)

// CurrencyValue represents the amount and variety of currency
type CurrencyValue struct {
	Amount   *big.Int
	Currency CurrencyDefinition
}

func (c *CurrencyValue) MarshalJSON() ([]byte, error) {
	var value = struct {
		Amount   string             `json:"amount"`
		Currency CurrencyDefinition `json:"currency"`
	}{
		Amount:   "0",
		Currency: c.Currency,
	}
	if c.Amount != nil {
		value.Amount = c.Amount.String()
	}

	return json.Marshal(value)

}

func (c *CurrencyValue) UnmarshalJSON(b []byte) error {
	var value struct {
		Amount   string             `json:"amount"`
		Currency CurrencyDefinition `json:"currency"`
	}
	err := json.Unmarshal(b, &value)
	if err != nil {
		return err
	}
	amt, ok := new(big.Int).SetString(value.Amount, 10)
	if !ok {
		return fmt.Errorf("invalid amount (%s)", value.Amount)
	}

	c.Amount = amt
	c.Currency = value.Currency
	return err
}

// NewCurrencyValueWithLookup accepts a string value as a base10 integer
// and uses the currency code to lookup the CurrencyDefinition
func NewCurrencyValueWithLookup(amount, currencyCode string) (*CurrencyValue, error) {
	def, err := AllCurrencies().Lookup(currencyCode)
	if err != nil {
		return nil, err
	}
	if amount == "" {
		return NewCurrencyValue("0", def)
	}
	return NewCurrencyValue(amount, def)
}

// NewCurrencyValueFromProtobuf consumes the string and pb.CurrencyDefinition
// objects from parsed Listings and converts them into CurrencyValue objects
func NewCurrencyValueFromProtobuf(amount string, currency *pb.CurrencyDefinition) (*CurrencyValue, error) {
	if currency == nil {
		return nil, ErrCurrencyDefinitionUndefined
	}
	value, err := NewCurrencyValueWithLookup(amount, currency.Code)
	if err != nil {
		return nil, err
	}
	value.Currency.Divisibility = uint(currency.Divisibility)
	return value, nil
}

// NewCurrencyValueFromInt is a convenience function which converts an int64
// into a string and passes the arguments to NewCurrencyValue
func NewCurrencyValueFromInt(amount int64, currency CurrencyDefinition) (*CurrencyValue, error) {
	return NewCurrencyValue(strconv.FormatInt(amount, 10), currency)
}

// NewCurrencyValueFromUint is a convenience function which converts an int64
// into a string and passes the arguments to NewCurrencyValue
func NewCurrencyValueFromUint(amount uint64, currency CurrencyDefinition) (*CurrencyValue, error) {
	return NewCurrencyValue(strconv.FormatUint(amount, 10), currency)
}

// NewCurrencyValue accepts string amounts and currency codes, and creates
// a valid CurrencyValue
func NewCurrencyValue(amount string, currency CurrencyDefinition) (*CurrencyValue, error) {
	var i, ok = new(big.Int).SetString(amount, 10)
	if !ok {
		return nil, ErrCurrencyValueAmountInvalid
	}
	return &CurrencyValue{Amount: i, Currency: currency}, nil
}

// AmountInt64 returns a valid int64 or an error
func (c *CurrencyValue) AmountInt64() (int64, error) {
	if !c.Amount.IsInt64() {
		return 0, ErrCurrencyValueInsufficientPrecision
	}
	return c.Amount.Int64(), nil
}

// AmountUint64 returns a valid int64 or an error
func (c *CurrencyValue) AmountUint64() (uint64, error) {
	if !c.Amount.IsUint64() {
		return 0, ErrCurrencyValueInsufficientPrecision
	}
	return c.Amount.Uint64(), nil
}

// AmountString returns the string representation of the amount
func (c *CurrencyValue) AmountString() string {
	if c == nil || c.Amount == nil {
		return "0"
	}
	return c.Amount.String()
}

// String returns a string representation of a CurrencyValue
func (c *CurrencyValue) String() string {
	if c == nil {
		return new(CurrencyValue).String()
	}
	return fmt.Sprintf("%s %s", c.Amount.String(), c.Currency.String())
}

// Valid returns an error if the CurrencyValue is invalid
func (c *CurrencyValue) Valid() error {
	if c.Amount == nil {
		return ErrCurrencyValueAmountInvalid
	}
	if err := c.Currency.Valid(); err != nil {
		return err
	}
	return nil
}

// Equal indicates if the amount and variety of currency is equivalent
func (c *CurrencyValue) Equal(other *CurrencyValue) bool {
	if c == nil && other == nil {
		return true
	}
	if c == nil || other == nil {
		return false
	}
	if !c.Currency.Equal(other.Currency) {
		if c.Currency.Code == other.Currency.Code {
			cN, err := c.Normalize()
			if err != nil {
				return false
			}
			oN, err := other.Normalize()
			if err != nil {
				return false
			}
			return cN.Amount.Cmp(oN.Amount) == 0
		}
		return false
	}
	return c.Amount.Cmp(other.Amount) == 0
}

// Normalize updates the CurrencyValue to match the divisibility of the locally defined CurrencyDefinition
func (c *CurrencyValue) Normalize() (*CurrencyValue, error) {
	localDef, err := AllCurrencies().Lookup(string(c.Currency.Code))
	if err != nil {
		return nil, err
	}
	val, _, err := c.AdjustDivisibility(localDef.Divisibility)
	return val, err
}

// AdjustDivisibility updates the Currency.Divisibility and adjusts the Amount to match the new
// value. An error will be returned if the new divisibility is invalid or produces an unreliable
// result. This is a helper function which is equivalent to ConvertTo using a copy of the
// CurrencyDefinition using the updated divisibility and an exchangeRatio of 1.0
func (c *CurrencyValue) AdjustDivisibility(div uint) (*CurrencyValue, big.Accuracy, error) {
	if c.Currency.Divisibility == div {
		return c, 0, nil
	}
	defWithNewDivisibility := c.Currency
	defWithNewDivisibility.Divisibility = div
	return c.ConvertTo(defWithNewDivisibility, 1.0)
}

// ConvertTo will perform the following math given its arguments are valid:
// c.Amount * exchangeRatio * (final.Currency.Divisibility/c.Currency.Divisibility)
// where c is the receiver, exchangeRatio is the ratio of (1 final.Currency/c.Currency)
// c and final must both be Valid() and exchangeRatio must not be zero. The accuracy
// indicates if decimal values were trimmed when converting the value back to integer.
func (c *CurrencyValue) ConvertTo(final CurrencyDefinition, exchangeRatio float64) (*CurrencyValue, big.Accuracy, error) {
	if err := c.Valid(); err != nil {
		return nil, 0, fmt.Errorf("cannot convert invalid value: %s", err.Error())
	}
	if err := final.Valid(); err != nil {
		return nil, 0, fmt.Errorf("cannot convert to invalid currency: %s", err.Error())
	}
	if exchangeRatio <= 0 {
		return nil, 0, ErrCurrencyValueNegativeRate
	}

	amt := new(big.Float).SetInt(c.Amount)
	exRatio := new(big.Float).SetFloat64(exchangeRatio)
	if exRatio == nil {
		return nil, 0, fmt.Errorf("exchange ratio (%f) is invalid", exchangeRatio)
	}
	newAmount := new(big.Float).SetPrec(53).Mul(amt, exRatio)

	if c.Currency.Divisibility != final.Divisibility {
		initMagnitude := math.Pow10(int(c.Currency.Divisibility))
		finalMagnitude := math.Pow10(int(final.Divisibility))
		divisibilityRatio := new(big.Float).SetFloat64(finalMagnitude / initMagnitude)
		newAmount.Mul(newAmount, divisibilityRatio)
	}

	var roundedAmount *big.Float
	newFloat, _ := newAmount.Float64()
	if newFloat >= 0 {
		roundedAmount = big.NewFloat(math.Ceil(newFloat))
	} else {
		roundedAmount = big.NewFloat(math.Floor(newFloat))
	}
	roundedInt, _ := roundedAmount.Int(nil)
	roundedAcc := big.Accuracy(roundedAmount.Cmp(newAmount))
	return &CurrencyValue{Amount: roundedInt, Currency: final}, roundedAcc, nil
}

// Cmp exposes the (*big.Int).Cmp behavior after verifying currency and adjusting
// for different currency divisibilities.
func (c *CurrencyValue) Cmp(other *CurrencyValue) (int, error) {
	if c.Currency.Code.String() != other.Currency.Code.String() {
		return 0, ErrCurrencyValueInvalidCmpDifferentCurrencies
	}
	if c.Currency.Equal(other.Currency) {
		return c.Amount.Cmp(other.Amount), nil
	}
	if c.Currency.Divisibility > other.Currency.Divisibility {
		adjOther, _, err := other.AdjustDivisibility(c.Currency.Divisibility)
		if err != nil {
			return 0, fmt.Errorf("adjusting other divisibility: %s", err.Error())
		}
		return c.Amount.Cmp(adjOther.Amount), nil
	}
	selfAdj, _, err := c.AdjustDivisibility(other.Currency.Divisibility)
	if err != nil {
		return 0, fmt.Errorf("adjusting self divisibility: %s", err.Error())
	}
	return selfAdj.Amount.Cmp(other.Amount), nil
}

// IsZero returns true if Amount is valid and equal to zero
func (c *CurrencyValue) IsZero() bool {
	if c.Amount == nil {
		return false
	}
	return c.Amount.Cmp(big.NewInt(0)) == 0
}

// IsNegative returns true if Amount is valid and less-than zero
func (c *CurrencyValue) IsNegative() bool {
	if c.Amount == nil {
		return false
	}
	return c.Amount.Cmp(big.NewInt(0)) == -1
}
