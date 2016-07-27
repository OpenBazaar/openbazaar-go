package bitcoin

type ExchangeRates interface {

	// Fetch the exchange rate for the given currency
	// It's OK if this returns from a cache.
	GetExchangeRate(currencyCode string) (float64, error)

	// Update the prices with the current exchange rate before returning.
	GetLatestRate(currencyCode string) (float64, error)
}
