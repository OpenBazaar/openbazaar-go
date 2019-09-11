package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
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
	Currency *CurrencyDefinition
}

func (c *CurrencyValue) MarshalJSON() ([]byte, error) {
	type currencyJson struct {
		Amount   string             `json:"amount"`
		Currency CurrencyDefinition `json:"currency"`
	}

	c0 := currencyJson{
		Amount:   "0",
		Currency: CurrencyDefinition{},
		//Amount:   c.Amount.String(),
		//Currency: *c.Currency,
	}

	if c.Amount != nil {
		c0.Amount = c.Amount.String()
	}

	if c.Currency != nil {
		c0.Currency = CurrencyDefinition{
			Code:         c.Currency.Code,
			Divisibility: c.Currency.Divisibility,
			Name:         c.Currency.Name,
			CurrencyType: c.Currency.CurrencyType,
		}
	}

	return json.Marshal(c0)

}

func (c *CurrencyValue) UnmarshalJSON(b []byte) error {
	type currencyJson struct {
		Amount   string             `json:"amount"`
		Currency CurrencyDefinition `json:"currency"`
	}

	var c0 currencyJson

	err := json.Unmarshal(b, &c0)
	if err == nil {
		c.Amount, _ = new(big.Int).SetString(c0.Amount, 10)
		//c.Currency, err = LoadCurrencyDefinitions().Lookup(c0.Currency.Code.String())
		c.Currency = &c0.Currency
	}

	return err
}

// NewCurrencyValueWithLookup accepts a string value as a base10 integer
// and uses the currency code to lookup the CurrencyDefinition
func NewCurrencyValueWithLookup(amount, currencyCode string) (*CurrencyValue, error) {
	def, err := LoadCurrencyDefinitions().Lookup(currencyCode)
	if err != nil {
		return nil, err
	}
	if amount == "" {
		return NewCurrencyValue("0", def)
	}
	return NewCurrencyValue(amount, def)
}

// NewCurrencyValueFromInt is a convenience function which converts an int64
// into a string and passes the arguments to NewCurrencyValue
func NewCurrencyValueFromInt(amount int64, currency *CurrencyDefinition) (*CurrencyValue, error) {
	return NewCurrencyValue(strconv.FormatInt(amount, 10), currency)
}

// NewCurrencyValueFromUint is a convenience function which converts an int64
// into a string and passes the arguments to NewCurrencyValue
func NewCurrencyValueFromUint(amount uint64, currency *CurrencyDefinition) (*CurrencyValue, error) {
	return NewCurrencyValue(strconv.FormatUint(amount, 10), currency)
}

// NewCurrencyValue accepts string amounts and currency codes, and creates
// a valid CurrencyValue
func NewCurrencyValue(amount string, currency *CurrencyDefinition) (*CurrencyValue, error) {
	var (
		i  = new(big.Int)
		ok bool
	)
	if _, ok = i.SetString(amount, 0); !ok {
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

// String returns a string representation of a CurrencyValue
func (v *CurrencyValue) String() string {
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
	if v == nil || other == nil {
		return false
	}
	if !v.Currency.Equal(other.Currency) {
		return false
	}
	return v.Amount.Cmp(other.Amount) == 0
}

// ConvertTo will perform the following math given its arguments are valid:
// v.Amount * exchangeRatio * (final.Currency.Divisibility/v.Currency.Divisibility)
// where v is the receiver, exchangeRatio is the ratio of (1 final.Currency/v.Currency)
// v and final must both be Valid() and exchangeRatio must not be zero.
func (v *CurrencyValue) ConvertTo(final *CurrencyDefinition, exchangeRatio float64) (*CurrencyValue, error) {
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
	newAmount := new(big.Float).Mul(amt, exRatio)

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
