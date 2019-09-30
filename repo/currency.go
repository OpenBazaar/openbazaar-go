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
	ErrCurrencyValueInsufficientPrecision = errors.New("unable to accurately represent value as int64")
	ErrCurrencyValueNegativeRate          = errors.New("conversion rate must be greater than zero")
	ErrCurrencyValueAmountInvalid         = errors.New("invalid amount")
	ErrCurrencyValueDefinitionInvalid     = errors.New("invalid currency definition")
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
	return v.AdjustDivisibility(localDef.Divisibility)
}

// AdjustDivisibility updates the Currency.Divisibility and adjusts the Amount to match the new
// value. An error will be returned if the new divisibility is invalid or produces an unreliable
// result. This is a helper function which is equivalent to ConvertTo using a copy of the
// CurrencyDefinition using the updated divisibility and an exchangeRatio of 1.0
func (v *CurrencyValue) AdjustDivisibility(div uint) (*CurrencyValue, error) {
	if v.Currency.Divisibility == div {
		return v, nil
	}
	defWithNewDivisibility := v.Currency
	defWithNewDivisibility.Divisibility = div
	return v.ConvertTo(defWithNewDivisibility, 1.0)
}

// ConvertTo will perform the following math given its arguments are valid:
// v.Amount * exchangeRatio * (final.Currency.Divisibility/v.Currency.Divisibility)
// where v is the receiver, exchangeRatio is the ratio of (1 final.Currency/v.Currency)
// v and final must both be Valid() and exchangeRatio must not be zero.
func (v *CurrencyValue) ConvertTo(final CurrencyDefinition, exchangeRatio float64) (*CurrencyValue, error) {
	if err := v.Valid(); err != nil {
		return nil, fmt.Errorf("cannot convert invalid value: %s", err.Error())
	}
	if err := final.Valid(); err != nil {
		return nil, fmt.Errorf("cannot convert to invalid currency: %s", err.Error())
	}
	if exchangeRatio <= 0 {
		return nil, ErrCurrencyValueNegativeRate
	}

	amt := new(big.Float).SetInt(v.Amount)
	exRatio := new(big.Float).SetFloat64(exchangeRatio)
	if exRatio == nil {
		return nil, fmt.Errorf("exchange ratio (%f) is invalid", exchangeRatio)
	}
	newAmount := new(big.Float).SetPrec(53).Mul(amt, exRatio)

	if v.Currency.Divisibility != final.Divisibility {
		initMagnitude := math.Pow10(int(v.Currency.Divisibility))
		finalMagnitude := math.Pow10(int(final.Divisibility))
		divisibilityRatio := new(big.Float).SetFloat64(finalMagnitude / initMagnitude)
		newAmount.Mul(newAmount, divisibilityRatio)

		// confirm no significant figures will be lost in the decimal
		if !newAmount.IsInt() {
			return nil, ErrCurrencyValueInsufficientPrecision
		}
	}

	result, _ := newAmount.Int(nil)
	return &CurrencyValue{Amount: result, Currency: final}, nil
}
