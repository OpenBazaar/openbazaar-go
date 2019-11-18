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
	ErrCurrencyValueNonPositiveRate               = errors.New("conversion rate must be greater than zero")
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
func (v *CurrencyValue) AmountInt64() (int64, error) {
	if !v.Amount.IsInt64() {
		return 0, ErrCurrencyValueInsufficientPrecision
	}
	return v.Amount.Int64(), nil
}

// AmountUint64 returns a valid int64 or an error
func (v *CurrencyValue) AmountUint64() (uint64, error) {
	if !v.Amount.IsUint64() {
		return 0, ErrCurrencyValueInsufficientPrecision
	}
	return v.Amount.Uint64(), nil
}

// AmountString returns the string representation of the amount
func (v *CurrencyValue) AmountString() string {
	if v == nil || v.Amount == nil {
		return "0"
	}
	return v.Amount.String()
}

// String returns a string representation of a CurrencyValue
func (v *CurrencyValue) String() string {
	if v == nil {
		return new(CurrencyValue).String()
	}
	return fmt.Sprintf("%s %s", v.Amount.String(), v.Currency.String())
}

// Valid returns an error if the CurrencyValue is invalid
func (v *CurrencyValue) Valid() error {
	if v.Amount == nil {
		return ErrCurrencyValueAmountInvalid
	}
	if err := v.Currency.Valid(); err != nil {
		return err
	}
	return nil
}

// Equal indicates if the amount and variety of currency is equivalent
func (v *CurrencyValue) Equal(other *CurrencyValue) bool {
	if v == nil && other == nil {
		return true
	}
	if v == nil || other == nil {
		return false
	}
	if !v.Currency.Equal(other.Currency) {
		if v.Currency.Code == other.Currency.Code {
			vN, err := v.Normalize()
			if err != nil {
				return false
			}
			oN, err := other.Normalize()
			if err != nil {
				return false
			}
			return vN.Amount.Cmp(oN.Amount) == 0
		}
		return false
	}
	return v.Amount.Cmp(other.Amount) == 0
}

// Normalize updates the CurrencyValue to match the divisibility of the locally defined CurrencyDefinition
func (v *CurrencyValue) Normalize() (*CurrencyValue, error) {
	localDef, err := AllCurrencies().Lookup(string(v.Currency.Code))
	if err != nil {
		return nil, err
	}
	val, _, err := v.AdjustDivisibility(localDef.Divisibility)
	return val, err
}

// AdjustDivisibility updates the Currency.Divisibility and adjusts the Amount to match the new
// value. An error will be returned if the new divisibility is invalid or produces an unreliable
// result. This is a helper function which is equivalent to ConvertTo using a copy of the
// CurrencyDefinition using the updated divisibility and an exchangeRatio of 1.0
func (v *CurrencyValue) AdjustDivisibility(div uint) (*CurrencyValue, big.Accuracy, error) {
	if v.Currency.Divisibility == div {
		return v, 0, nil
	}
	defWithNewDivisibility := v.Currency
	defWithNewDivisibility.Divisibility = div
	return v.ConvertTo(defWithNewDivisibility, NewEqualExchangeRater())
}

// ConvertTo will perform the following math given its arguments are valid:
// v.Amount * exchangeRatio * (final.Currency.Divisibility/v.Currency.Divisibility)
// where v is the receiver, exchangeRatio is the ratio of (1 final.Currency/v.Currency)
// v and final must both be Valid() and exchangeRatio must not be zero. The accuracy
// indicates if decimal values were trimmed when converting the value back to integer.
func (v *CurrencyValue) ConvertTo(final CurrencyDefinition, exRater ExchangeRater) (*CurrencyValue, big.Accuracy, error) {
	//func (v *CurrencyValue) ConvertTo(final CurrencyDefinition, exchangeRatio float64) (*CurrencyValue, big.Accuracy, error) {
	if err := v.Valid(); err != nil {
		return nil, 0, fmt.Errorf("cannot convert invalid value: %s", err.Error())
	}
	if err := final.Valid(); err != nil {
		return nil, 0, fmt.Errorf("cannot convert to invalid currency: %s", err.Error())
	}
	inRate, err := exRater.GetExchangeRate(v.Currency.Code.String())
	if err != nil {
		return nil, 0, fmt.Errorf("looking up (%s) rate: %s", v.Currency.Code.String(), err.Error())
	}
	outRate, err := exRater.GetExchangeRate(final.Code.String())
	if err != nil {
		return nil, 0, fmt.Errorf("looking up (%s) rate: %s", final.Code.String(), err.Error())
	}
	rate := 1 / (inRate / outRate)
	if rate <= 0 {
		return nil, 0, ErrCurrencyValueNonPositiveRate
	}
	exRatio := new(big.Float).SetFloat64(rate)

	amt := new(big.Float).SetInt(v.Amount)
	newAmount := new(big.Float).SetPrec(53).Mul(amt, exRatio)

	if v.Currency.Divisibility != final.Divisibility {
		initMagnitude := math.Pow10(int(v.Currency.Divisibility))
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
func (v *CurrencyValue) Cmp(other *CurrencyValue) (int, error) {
	if v.Currency.Code.String() != other.Currency.Code.String() {
		return 0, ErrCurrencyValueInvalidCmpDifferentCurrencies
	}
	if v.Currency.Equal(other.Currency) {
		return v.Amount.Cmp(other.Amount), nil
	}
	if v.Currency.Divisibility > other.Currency.Divisibility {
		adjOther, _, err := other.AdjustDivisibility(v.Currency.Divisibility)
		if err != nil {
			return 0, fmt.Errorf("adjusting other divisibility: %s", err.Error())
		}
		return v.Amount.Cmp(adjOther.Amount), nil
	}
	selfAdj, _, err := v.AdjustDivisibility(other.Currency.Divisibility)
	if err != nil {
		return 0, fmt.Errorf("adjusting self divisibility: %s", err.Error())
	}
	return selfAdj.Amount.Cmp(other.Amount), nil
}
