package spvwallet

import (
	"encoding/json"
	"golang.org/x/net/proxy"
	"net"
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
	feeAPI      string

	httpClient httpClient

	cache *feeCache
}

func NewFeeProvider(maxFee, priorityFee, normalFee, economicFee uint64, feeAPI string, proxy proxy.Dialer) *FeeProvider {
	fp := FeeProvider{
		maxFee:      maxFee,
		priorityFee: priorityFee,
		normalFee:   normalFee,
		economicFee: economicFee,
		feeAPI:      feeAPI,
		cache:       new(feeCache),
	}
	dial := net.Dial
	if proxy != nil {
		dial = proxy.Dial
	}
	tbTransport := &http.Transport{Dial: dial}
	httpClient := &http.Client{Transport: tbTransport, Timeout: time.Second * 10}
	fp.httpClient = httpClient
	return &fp
}

func (fp *FeeProvider) GetFeePerByte(feeLevel FeeLevel) uint64 {
	defaultFee := func() uint64 {
		switch feeLevel {
		case PRIOIRTY:
			return fp.priorityFee
		case NORMAL:
			return fp.normalFee
		case ECONOMIC:
			return fp.economicFee
		case FEE_BUMP:
			return fp.priorityFee * 2
		default:
			return fp.normalFee
		}
	}
	if fp.feeAPI == "" {
		return defaultFee()
	}
	fees := new(Fees)
	if time.Since(fp.cache.lastUpdated) > time.Minute {
		resp, err := fp.httpClient.Get(fp.feeAPI)
		if err != nil {
			return defaultFee()
		}

		defer resp.Body.Close()

		err = json.NewDecoder(resp.Body).Decode(&fees)
		if err != nil {
			return defaultFee()
		}
		fp.cache.lastUpdated = time.Now()
		fp.cache.fees = fees
	} else {
		fees = fp.cache.fees
	}
	switch feeLevel {
	case PRIOIRTY:
		if fees.FastestFee > fp.maxFee || fees.FastestFee == 0 {
			return fp.maxFee
		} else {
			return fees.FastestFee
		}
	case NORMAL:
		if fees.HalfHourFee > fp.maxFee || fees.HalfHourFee == 0 {
			return fp.maxFee
		} else {
			return fees.HalfHourFee
		}
	case ECONOMIC:
		if fees.HourFee > fp.maxFee || fees.HourFee == 0 {
			return fp.maxFee
		} else {
			return fees.HourFee
		}
	case FEE_BUMP:
		if (fees.FastestFee*2) > fp.maxFee || fees.FastestFee == 0 {
			return fp.maxFee
		} else {
			return fees.FastestFee * 2
		}
	default:
		return fp.normalFee
	}
}
