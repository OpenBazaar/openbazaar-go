package repo

// ExchangeRater is used to access currency rates against BTC
type ExchangeRater interface {
	// GetExchangeRate finds the exchange rate of the currencyCode as it
	// compares to BTC
	GetExchangeRate(currencyCode string) (float64, error)
}

type equalExchangeRater struct{}

func (*equalExchangeRater) GetExchangeRate(_ string) (float64, error) {
	return 1.0, nil
}

// NewEqualExchangeRater creates an exchange rater where all
// requested rates are returned at 1.0, effectively making
// the ExchangeRater an ignored variable for testing purposes
func NewEqualExchangeRater() *equalExchangeRater {
	return &equalExchangeRater{}
}

//func GetWalletExchangeRater(mw multiwallet.Multiwallet, code string) (ExchangeRater, error) {
//return nil, nil
//}
