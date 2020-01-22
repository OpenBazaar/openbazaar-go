package exchangerates

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/op/go-logging"
	"golang.org/x/net/proxy"
)

const SatoshiPerBTC int64 = 100000000

var log = logging.MustGetLogger("exchangeRates")

type ExchangeRateProvider struct {
	fetchUrl string
	cache    map[string]float64
	client   *http.Client
	decoder  ExchangeRateDecoder
}

type ExchangeRateDecoder interface {
	decode(dat interface{}, cache map[string]float64) (err error)
}

// empty structs to tag the different ExchangeRateDecoder implementations
type BitcoinAverageDecoder struct{}
type BitPayDecoder struct{}
type BlockchainInfoDecoder struct{}
type BitcoinChartsDecoder struct{}

type BitcoinPriceFetcher struct {
	sync.Mutex
	cache     map[string]float64
	providers []*ExchangeRateProvider
}

func NewBitcoinPriceFetcher(dialer proxy.Dialer) *BitcoinPriceFetcher {
	b := BitcoinPriceFetcher{
		cache: make(map[string]float64),
	}
	dial := net.Dial
	if dialer != nil {
		dial = dialer.Dial
	}
	tbTransport := &http.Transport{Dial: dial}
	client := &http.Client{Transport: tbTransport, Timeout: time.Minute}

	b.providers = []*ExchangeRateProvider{
		{"https://ticker.openbazaar.org/api", b.cache, client, BitcoinAverageDecoder{}},
		{"https://bitpay.com/api/rates", b.cache, client, BitPayDecoder{}},
		{"https://blockchain.info/ticker", b.cache, client, BlockchainInfoDecoder{}},
		{"https://api.bitcoincharts.com/v1/weighted_prices.json", b.cache, client, BitcoinChartsDecoder{}},
	}
	return &b
}

func (b *BitcoinPriceFetcher) GetExchangeRate(currencyCode string) (float64, error) {
	currencyCode = NormalizeCurrencyCode(currencyCode)

	b.Lock()
	defer b.Unlock()
	price, ok := b.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (b *BitcoinPriceFetcher) GetLatestRate(currencyCode string) (float64, error) {
	currencyCode = NormalizeCurrencyCode(currencyCode)

	b.fetchCurrentRates()
	b.Lock()
	defer b.Unlock()
	price, ok := b.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (b *BitcoinPriceFetcher) GetAllRates(cacheOK bool) (map[string]float64, error) {
	if !cacheOK {
		err := b.fetchCurrentRates()
		if err != nil {
			return nil, err
		}
	}
	b.Lock()
	defer b.Unlock()
	copy := make(map[string]float64, len(b.cache))
	for k, v := range b.cache {
		copy[k] = v
	}
	return copy, nil
}

func (b *BitcoinPriceFetcher) UnitsPerCoin() int64 {
	return SatoshiPerBTC
}

func (b *BitcoinPriceFetcher) fetchCurrentRates() error {
	b.Lock()
	defer b.Unlock()
	for _, provider := range b.providers {
		err := provider.fetch()
		if err == nil {
			return nil
		}
	}
	log.Error("Failed to fetch bitcoin exchange rates")
	return errors.New("All exchange rate API queries failed")
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
	return provider.decoder.decode(dataMap, provider.cache)
}

func (b *BitcoinPriceFetcher) Run() {
	b.fetchCurrentRates()
	ticker := time.NewTicker(time.Minute * 15)
	for range ticker.C {
		b.fetchCurrentRates()
	}
}

// Decoders
func (b BitcoinAverageDecoder) decode(dat interface{}, cache map[string]float64) (err error) {
	data, ok := dat.(map[string]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed")
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
			cache[k] = price
		}
	}
	return nil
}

func (b BitPayDecoder) decode(dat interface{}, cache map[string]float64) (err error) {
	data, ok := dat.([]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed, not JSON array")
	}

	for _, obj := range data {
		code := obj.(map[string]interface{})
		k, ok := code["code"].(string)
		if !ok {
			return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed, missing 'code' (string) field")
		}
		price, ok := code["rate"].(float64)
		if !ok {
			return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed, missing 'rate' (float) field")
		}
		cache[k] = price
	}
	return nil
}

func (b BlockchainInfoDecoder) decode(dat interface{}, cache map[string]float64) (err error) {
	data, ok := dat.(map[string]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed, not JSON object")
	}
	for k, v := range data {
		val, ok := v.(map[string]interface{})
		if !ok {
			return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed")
		}
		price, ok := val["last"].(float64)
		if !ok {
			return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed, missing 'last' (float) field")
		}
		cache[k] = price
	}
	return nil
}

func (b BitcoinChartsDecoder) decode(dat interface{}, cache map[string]float64) (err error) {
	data, ok := dat.(map[string]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed, not JSON object")
	}
	for k, v := range data {
		if k != "timestamp" {
			val, ok := v.(map[string]interface{})
			if !ok {
				return errors.New("Type assertion failed")
			}
			p, ok := val["24h"]
			if !ok {
				continue
			}
			pr, ok := p.(string)
			if !ok {
				return errors.New("Type assertion failed")
			}
			price, err := strconv.ParseFloat(pr, 64)
			if err != nil {
				return err
			}
			cache[k] = price
		}
	}
	return nil
}

// NormalizeCurrencyCode standardizes the format for the given currency code
func NormalizeCurrencyCode(currencyCode string) string {
	return strings.ToUpper(currencyCode)
}
