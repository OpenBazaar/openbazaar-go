package repo

import (
	"errors"
	"math/big"
	"strings"

	"github.com/OpenBazaar/wallet-interface"
)

var (
	// ErrPriceCalculationRequiresExchangeRates - exchange rates dependency err
	ErrPriceCalculationRequiresExchangeRates = errors.New("can't calculate price with exchange rates disabled")
)

// CurrencyConverter
type CurrencyConverter struct {
	ReserveWallet wallet.Wallet
}

func NewCurrencyConverter(reserveWallet wallet.Wallet) CurrencyConverter {
	return CurrencyConverter{
		reserveWallet,
	}
}

func (c *CurrencyConverter) GetReserveToCurrencyRate(destinationCurrencyCode string) (*big.Float, error) {
	if c.ReserveWallet.ExchangeRates() == nil {
		return big.NewFloat(0), ErrPriceCalculationRequiresExchangeRates
	}

	reserveIntoOriginRate, err := c.ReserveWallet.ExchangeRates().GetExchangeRate(destinationCurrencyCode)
	if err != nil {
		// TODO: remove hack once ExchangeRates can be made aware of testnet currencies
		if strings.HasPrefix(destinationCurrencyCode, "T") {
			reserveIntoOriginRate, err = c.ReserveWallet.ExchangeRates().GetExchangeRate(strings.TrimPrefix(destinationCurrencyCode, "T"))
			if err != nil {
				return big.NewFloat(0), err
			}
		} else {
			return big.NewFloat(0), err
		}
	}

	// Convert to big.Float
	return new(big.Float).SetFloat64(reserveIntoOriginRate), nil
}

func (c *CurrencyConverter) GetConversionRate(originCurrencyCode string, destinationCurrencyCode string) (*big.Float, error) {
	originRate, err := c.GetReserveToCurrencyRate(originCurrencyCode)
	if err != nil {
		return big.NewFloat(0), err
	}

	paymentRate, err := c.GetReserveToCurrencyRate(destinationCurrencyCode)
	if err != nil {
		return big.NewFloat(0), err
	}

	return new(big.Float).Quo(paymentRate, originRate), nil
}

func (c *CurrencyConverter) GetFinalPrice(originCurrency CurrencyDefinition, destinationCurrency CurrencyDefinition, amount *big.Int) (*big.Float, error) {
	// 1. Get conversion rates from reserve currency to origin/payment
	// 2. Divide the conversion rates (payment rate / origin rate)
	// 3. Multiply original amount * conversion rate

	finalRate, err := c.GetConversionRate(originCurrency.Code.String(), destinationCurrency.Code.String())
	if err != nil {
		return nil, err
	}
	log.Debugf("Final Rate: %s", finalRate.String())

	// Amounts priced in fiat will come in lowest unit not in whole items so we need
	// to convert it to the priced magnitude
	divisibility := new(big.Int).SetUint64(uint64(originCurrency.Divisibility))
	divisibilityFactor := new(big.Int).Exp(new(big.Int).SetInt64(10), divisibility, nil)
	amountFloat := new(big.Float).SetInt(amount)
	divisibilityFloat := new(big.Float).SetInt(divisibilityFactor)
	originalAmount := new(big.Float).Quo(amountFloat, divisibilityFloat)

	log.Debugf("Original Amount: %s", originalAmount.String())
	finalPrice := new(big.Float).Mul(finalRate, originalAmount)
	log.Debugf("Final Price: %s", finalPrice.String())

	return finalPrice, nil
}

func getDivisibilityConversionFactor(div1 uint, div2 uint) *big.Int {
	deltaExponent := int64(div1) - int64(div2)
	deltaExponent64 := new(big.Int).SetInt64(deltaExponent)
	return new(big.Int).Exp(new(big.Int).SetInt64(10), deltaExponent64, nil)
}
