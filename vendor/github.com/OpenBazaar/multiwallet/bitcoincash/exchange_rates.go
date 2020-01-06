package bitcoincash

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/OpenBazaar/multiwallet/util"
	"golang.org/x/net/proxy"
	"net/http"
	"reflect"
	"sync"
	"time"
)

type ExchangeRateProvider struct {
	fetchUrl string
	cache    map[string]float64
	client   *http.Client
	decoder  ExchangeRateDecoder
}

type ExchangeRateDecoder interface {
	decode(dat interface{}, cache map[string]float64) (err error)
}

type OpenBazaarDecoder struct{}

type BitcoinCashPriceFetcher struct {
	sync.Mutex
	cache     map[string]float64
	providers []*ExchangeRateProvider
}

func NewBitcoinCashPriceFetcher(dialer proxy.Dialer) *BitcoinCashPriceFetcher {
	b := BitcoinCashPriceFetcher{
		cache: make(map[string]float64),
	}
	var client *http.Client
	if dialer != nil {
		dial := dialer.Dial
		tbTransport := &http.Transport{Dial: dial}
		client = &http.Client{Transport: tbTransport, Timeout: time.Minute}
	} else {
		client = &http.Client{Timeout: time.Minute}
	}


	b.providers = []*ExchangeRateProvider{
		{"https://ticker.openbazaar.org/api", b.cache, client, OpenBazaarDecoder{}},
	}
	return &b
}

func (b *BitcoinCashPriceFetcher) GetExchangeRate(currencyCode string) (float64, error) {
	b.Lock()
	defer b.Unlock()

	currencyCode = util.NormalizeCurrencyCode(currencyCode)
	price, ok := b.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (b *BitcoinCashPriceFetcher) GetLatestRate(currencyCode string) (float64, error) {
	b.fetchCurrentRates()
	b.Lock()
	defer b.Unlock()

	currencyCode = util.NormalizeCurrencyCode(currencyCode)
	price, ok := b.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (b *BitcoinCashPriceFetcher) GetAllRates(cacheOK bool) (map[string]float64, error) {
	if !cacheOK {
		err := b.fetchCurrentRates()
		if err != nil {
			return nil, err
		}
	}
	b.Lock()
	defer b.Unlock()
	return b.cache, nil
}

func (b *BitcoinCashPriceFetcher) UnitsPerCoin() int {
	return 100000000
}

func (b *BitcoinCashPriceFetcher) fetchCurrentRates() error {
	b.Lock()
	defer b.Unlock()
	for _, provider := range b.providers {
		err := provider.fetch()
		if err == nil {
			return nil
		}
		fmt.Println(err)
	}
	return errors.New("All exchange rate API queries failed")
}

func (b *BitcoinCashPriceFetcher) Run() {
	b.fetchCurrentRates()
	ticker := time.NewTicker(time.Minute * 15)
	for range ticker.C {
		b.fetchCurrentRates()
	}
}

func (provider *ExchangeRateProvider) fetch() (err error) {
	if len(provider.fetchUrl) == 0 {
		err = errors.New("provider has no fetchUrl")
		return err
	}
	resp, err := provider.client.Get(provider.fetchUrl)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(resp.Body)
	var dataMap interface{}
	err = decoder.Decode(&dataMap)
	if err != nil {
		return err
	}
	return provider.decoder.decode(dataMap, provider.cache)
}

func (b OpenBazaarDecoder) decode(dat interface{}, cache map[string]float64) (err error) {
	data, ok := dat.(map[string]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed")
	}
	bch, ok := data["BCH"]
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed, missing 'BCH' field")
	}
	val, ok := bch.(map[string]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed")
	}
	bchRate, ok := val["last"].(float64)
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
			cache[k] = price * (1 / bchRate)
		}
	}
	return nil
}
