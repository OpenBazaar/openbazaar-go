package zcoind

import (
	"encoding/json"
	"errors"
	"github.com/OpenBazaar/openbazaar-go/bitcoin/exchange"
	"golang.org/x/net/proxy"
	"net"
	"net/http"
	"reflect"
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

type OpenBazaarDecoder struct{}
type BittrexDecoder struct{}

type zcoinPriceFetcher struct {
	sync.Mutex
	cache     map[string]float64
	providers []*ExchangeRateProvider
}

func NewzcoinPriceFetcher(dialer proxy.Dialer) *zcoinPriceFetcher {
	bp := exchange.NewBitcoinPriceFetcher(dialer)
	z := zcoinPriceFetcher{
		cache: make(map[string]float64),
	}
	dial := net.Dial
	if dialer != nil {
		dial = dialer.Dial
	}
	tbTransport := &http.Transport{Dial: dial}
	client := &http.Client{Transport: tbTransport, Timeout: time.Minute}

	z.providers = []*ExchangeRateProvider{
		{"https://ticker.openbazaar.org/api", z.cache, client, OpenBazaarDecoder{}, nil},
		{"https://bittrex.com/api/v1.1/public/getticker?market=btc-xzc", z.cache, client, BittrexDecoder{}, bp},
	}
	go z.run()
	return &z
}

func (z *zcoinPriceFetcher) GetExchangeRate(currencyCode string) (float64, error) {
	z.Lock()
	defer z.Unlock()
	price, ok := z.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (z *zcoinPriceFetcher) GetLatestRate(currencyCode string) (float64, error) {
	z.fetchCurrentRates()
	z.Lock()
	defer z.Unlock()
	price, ok := z.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (z *zcoinPriceFetcher) GetAllRates(cacheOK bool) (map[string]float64, error) {
	if !cacheOK {
		err := z.fetchCurrentRates()
		if err != nil {
			return nil, err
		}
	}
	z.Lock()
	defer z.Unlock()
	return z.cache, nil
}

func (z *zcoinPriceFetcher) UnitsPerCoin() int {
	return exchange.SatoshiPerBTC
}

func (z *zcoinPriceFetcher) fetchCurrentRates() error {
	z.Lock()
	defer z.Unlock()
	for _, provider := range z.providers {
		err := provider.fetch()
		if err == nil {
			return nil
		}
	}
	log.Error("Failed to fetch zcoin exchange rates")
	return errors.New("All exchange rate API queries failed")
}

func (z *zcoinPriceFetcher) run() {
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

func (b OpenBazaarDecoder) decode(dat interface{}, cache map[string]float64, bp *exchange.BitcoinPriceFetcher) (err error) {
	data := dat.(map[string]interface{})

	xzc, ok := data["XZC"]
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed, missing 'XZC' field")
	}
	val, ok := xzc.(map[string]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed")
	}
	xzcRate, ok := val["last"].(float64)
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed, missing 'last' (float) field")
	}
	for k, v := range data {
		if k != "timestamp" {
			val, ok := v.(map[string]interface{})
			if !ok {
				return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed")
			}
			price, ok := val["last"].(float64)
			if !ok {
				return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed, missing 'last' (float) field")
			}
			cache[k] = price * (1 / xzcRate)
		}
	}
	return nil
}

func (b BittrexDecoder) decode(dat interface{}, cache map[string]float64, bp *exchange.BitcoinPriceFetcher) (err error) {
	rates, err := bp.GetAllRates(false)
	if err != nil {
		return err
	}
	obj, ok := dat.(map[string]interface{})
	if !ok {
		return errors.New("BittrexDecoder type assertion failure")
	}
	result, ok := obj["result"]
	if !ok {
		return errors.New("BittrexDecoder: field `result` not found")
	}
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return errors.New("BittrexDecoder type assertion failure")
	}
	exRate, ok := resultMap["Last"]
	if !ok {
		return errors.New("BittrexDecoder: field `Last` not found")
	}
	rate, ok := exRate.(float64)
	if !ok {
		return errors.New("BittrexDecoder type assertion failure")
	}

	if rate == 0 {
		return errors.New("Bitcoin-zcoin price data not available")
	}
	for k, v := range rates {
		cache[k] = v * rate
	}
	return nil
}
