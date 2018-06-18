package bitcoincash

import (
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"github.com/OpenBazaar/wallet-interface"
	"net/http"
	"time"
)

type httpClient interface {
	Get(string) (*http.Response, error)
}

type feeCache struct {
	fees        *Fees
	lastUpdated time.Time
}

type Fees struct {
	FastestFee  uint64
	HalfHourFee uint64
	HourFee     uint64
}

type FeeProvider struct {
	maxFee      uint64
	priorityFee uint64
	normalFee   uint64
	economicFee uint64

	exchangeRates bitcoin.ExchangeRates

	cache *feeCache
}

// We will target a fee per byte such that it would equal
// 1 USD cent for economic, 5 USD cents for normal and
// 10 USD cents for priority for a median (226 byte) transaction.
type FeeTarget int

const (
	EconomicTarget FeeTarget = 1
	NormalTarget   FeeTarget = 5
	PriorityTarget FeeTarget = 10
)

func NewFeeProvider(maxFee, priorityFee, normalFee, economicFee uint64, exchangeRates bitcoin.ExchangeRates) *FeeProvider {
	return &FeeProvider{
		maxFee:        maxFee,
		priorityFee:   priorityFee,
		normalFee:     normalFee,
		economicFee:   economicFee,
		exchangeRates: exchangeRates,
		cache:         new(feeCache),
	}
}

func (fp *FeeProvider) GetFeePerByte(feeLevel wallet.FeeLevel) uint64 {
	defaultFee := func() uint64 {
		switch feeLevel {
		case wallet.PRIOIRTY:
			return fp.priorityFee
		case wallet.NORMAL:
			return fp.normalFee
		case wallet.ECONOMIC:
			return fp.economicFee
		case wallet.FEE_BUMP:
			return fp.priorityFee * 2
		default:
			return fp.normalFee
		}
	}
	if fp.exchangeRates == nil {
		return defaultFee()
	}

	rate, err := fp.exchangeRates.GetLatestRate("USD")
	if err != nil || rate == 0 {
		log.Errorf("Error using exchange rate to calculate fee: %s\n", err.Error())
		return defaultFee()
	}

	var target FeeTarget
	switch feeLevel {
	case wallet.PRIOIRTY:
		target = PriorityTarget
	case wallet.NORMAL:
		target = NormalTarget
	case wallet.ECONOMIC:
		target = EconomicTarget
	case wallet.FEE_BUMP:
		target = PriorityTarget * 2
	default:
		target = NormalTarget
	}

	feePerByte := (((float64(target) / 100) / rate) * 100000000) / 226

	if uint64(feePerByte) > fp.maxFee {
		return fp.maxFee
	}

	return uint64(feePerByte)
}
