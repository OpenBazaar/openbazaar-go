package spvwallet

import (
	"encoding/json"
	"github.com/OpenBazaar/wallet-interface"
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
	Priority uint64 `json:"priority"`
	Normal   uint64 `json:"normal"`
	Economic uint64 `json:"economic"`
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
	case wallet.PRIOIRTY:
		if fees.Priority > fp.maxFee || fees.Priority == 0 {
			return fp.maxFee
		} else {
			return fees.Priority
		}
	case wallet.NORMAL:
		if fees.Normal > fp.maxFee || fees.Normal == 0 {
			return fp.maxFee
		} else {
			return fees.Normal
		}
	case wallet.ECONOMIC:
		if fees.Economic > fp.maxFee || fees.Economic == 0 {
			return fp.maxFee
		} else {
			return fees.Economic
		}
	case wallet.FEE_BUMP:
		if (fees.Priority*2) > fp.maxFee || fees.Priority == 0 {
			return fp.maxFee
		} else {
			return fees.Priority * 2
		}
	default:
		return fp.normalFee
	}
}
