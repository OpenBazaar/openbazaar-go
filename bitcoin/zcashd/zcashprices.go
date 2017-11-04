package zcashd

import (
	"encoding/json"
	"errors"
	"github.com/OpenBazaar/openbazaar-go/bitcoin/exchange"
	"golang.org/x/net/proxy"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"
)

type ExchangeRateProvider struct {
	fetchUrl        string
	cache           map[string]float64
	client          *http.Client
	decoder         ExchangeRateDecoder
	bitcoinProvider *exchange.BitcoinPriceFetcher
}

type ExchangeRateDecoder interface {
	decode(dat interface{}, cache map[string]float64, bp *exchange.BitcoinPriceFetcher) (err error)
}

type PoloniexDecoder struct{}

type ZcashPriceFetcher struct {
	sync.Mutex
	cache     map[string]float64
	providers []*ExchangeRateProvider
}

func NewZcashPriceFetcher(dialer proxy.Dialer) *ZcashPriceFetcher {
	bp := exchange.NewBitcoinPriceFetcher(dialer)
	z := ZcashPriceFetcher{
		cache: make(map[string]float64),
	}
	dial := net.Dial
	if dialer != nil {
		dial = dialer.Dial
	}
	tbTransport := &http.Transport{Dial: dial}
	client := &http.Client{Transport: tbTransport, Timeout: time.Minute}

	z.providers = []*ExchangeRateProvider{
		{"https://poloniex.com/public?command=returnTicker", z.cache, client, PoloniexDecoder{}, bp},
	}
	go z.run()
	return &z
}

func (z *ZcashPriceFetcher) GetExchangeRate(currencyCode string) (float64, error) {
	z.Lock()
	defer z.Unlock()
	price, ok := z.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (z *ZcashPriceFetcher) GetLatestRate(currencyCode string) (float64, error) {
	z.fetchCurrentRates()
	z.Lock()
	defer z.Unlock()
	price, ok := z.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (z *ZcashPriceFetcher) GetAllRates() (map[string]float64, error) {
	z.Lock()
	defer z.Unlock()
	return z.cache, nil
}

func (z *ZcashPriceFetcher) UnitsPerCoin() int {
	return exchange.SatoshiPerBTC
}

func (z *ZcashPriceFetcher) fetchCurrentRates() error {
	z.Lock()
	defer z.Unlock()
	for _, provider := range z.providers {
		err := provider.fetch()
		if err == nil {
			return nil
		}
	}
	log.Error("Failed to fetch zcash exchange rates")
	return errors.New("All exchange rate API queries failed")
}

func (z *ZcashPriceFetcher) run() {
	z.fetchCurrentRates()
	ticker := time.NewTicker(time.Minute * 15)
	for range ticker.C {
		z.fetchCurrentRates()
	}
}

func (provider *ExchangeRateProvider) fetch() (err error) {
	if len(provider.fetchUrl) == 0 {
		err = errors.New("Provider has no fetchUrl")
		return err
	}
	resp, err := provider.client.Get(provider.fetchUrl)
	if err != nil {
		log.Error("Failed to fetch from "+provider.fetchUrl, err)
		return err
	}
	decoder := json.NewDecoder(resp.Body)
	var dataMap interface{}
	err = decoder.Decode(&dataMap)
	if err != nil {
		log.Error("Failed to decode JSON from "+provider.fetchUrl, err)
		return err
	}
	return provider.decoder.decode(dataMap, provider.cache, provider.bitcoinProvider)
}

func (b PoloniexDecoder) decode(dat interface{}, cache map[string]float64, bp *exchange.BitcoinPriceFetcher) (err error) {
	rates, err := bp.GetAllRates()
	if err != nil {
		return err
	}
	data := dat.(map[string]interface{})
	var rate float64
	for k, v := range data {
		if k == "BTC_ZEC" {
			val, ok := v.(map[string]interface{})
			if !ok {
				return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed")
			}
			s, ok := val["last"].(string)
			if !ok {
				return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed, missing 'last' (string) field")
			}
			price, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return err
			}
			rate = price
		}
	}
	if rate == 0 {
		return errors.New("Bitcoin-Zcash price data not available")
	}
	for k, v := range rates {
		cache[k] = v * rate
	}
	return nil
}
