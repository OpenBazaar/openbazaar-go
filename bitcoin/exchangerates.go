package bitcoin

type ExchangeRates interface {

	// Fetch the exchange rate for the given currency
	// It's OK if this returns from a cache.
	GetExchangeRate(currencyCode string) (float64, error)

	// Update the prices with the current exchange rate before returning.
	GetLatestRate(currencyCode string) (float64, error)

	// Returns all available rates
	// It's OK if this returns from cache
	GetAllRates() (map[string]float64, error)

	// Return the number of currency units per coin. For example, in bitcoin
	// this is 100m satoshi per BTC. This is used when converting from fiat
	// to the smaller currency unit.
	UnitsPerCoin() int
}
