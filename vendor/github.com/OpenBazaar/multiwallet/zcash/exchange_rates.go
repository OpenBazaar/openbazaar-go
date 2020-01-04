package zcash

import (
	"encoding/json"
	"errors"
	"github.com/OpenBazaar/multiwallet/util"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"

	exchange "github.com/OpenBazaar/spvwallet/exchangerates"
	"golang.org/x/net/proxy"
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
type KrakenDecoder struct{}
type PoloniexDecoder struct{}
type BitfinexDecoder struct{}
type BittrexDecoder struct{}

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

	var client *http.Client
	if dialer != nil {
		dial := dialer.Dial
		tbTransport := &http.Transport{Dial: dial}
		client = &http.Client{Transport: tbTransport, Timeout: time.Minute}
	} else {
		client = &http.Client{Timeout: time.Minute}
	}


	z.providers = []*ExchangeRateProvider{
		{"https://ticker.openbazaar.org/api", z.cache, client, OpenBazaarDecoder{}, nil},
		{"https://bittrex.com/api/v1.1/public/getticker?market=btc-zec", z.cache, client, BittrexDecoder{}, bp},
		{"https://api.bitfinex.com/v1/pubticker/zecbtc", z.cache, client, BitfinexDecoder{}, bp},
		{"https://poloniex.com/public?command=returnTicker", z.cache, client, PoloniexDecoder{}, bp},
		{"https://api.kraken.com/0/public/Ticker?pair=ZECXBT", z.cache, client, KrakenDecoder{}, bp},
	}
	go z.run()
	return &z
}

func (z *ZcashPriceFetcher) GetExchangeRate(currencyCode string) (float64, error) {
	currencyCode = util.NormalizeCurrencyCode(currencyCode)

	z.Lock()
	defer z.Unlock()
	price, ok := z.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (z *ZcashPriceFetcher) GetLatestRate(currencyCode string) (float64, error) {
	currencyCode = util.NormalizeCurrencyCode(currencyCode)

	z.fetchCurrentRates()
	z.Lock()
	defer z.Unlock()
	price, ok := z.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (z *ZcashPriceFetcher) GetAllRates(cacheOK bool) (map[string]float64, error) {
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
		return err
	}
	decoder := json.NewDecoder(resp.Body)
	var dataMap interface{}
	err = decoder.Decode(&dataMap)
	if err != nil {
		return err
	}
	return provider.decoder.decode(dataMap, provider.cache, provider.bitcoinProvider)
}

func (b OpenBazaarDecoder) decode(dat interface{}, cache map[string]float64, bp *exchange.BitcoinPriceFetcher) (err error) {
	data, ok := dat.(map[string]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed")
	}

	zec, ok := data["ZEC"]
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed, missing 'ZEC' field")
	}
	val, ok := zec.(map[string]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed")
	}
	zecRate, ok := val["last"].(float64)
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
			cache[k] = price * (1 / zecRate)
		}
	}
	return nil
}

func (b KrakenDecoder) decode(dat interface{}, cache map[string]float64, bp *exchange.BitcoinPriceFetcher) (err error) {
	rates, err := bp.GetAllRates(false)
	if err != nil {
		return err
	}
	obj, ok := dat.(map[string]interface{})
	if !ok {
		return errors.New("KrackenDecoder type assertion failure")
	}
	result, ok := obj["result"]
	if !ok {
		return errors.New("KrakenDecoder: field `result` not found")
	}
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return errors.New("KrackenDecoder type assertion failure")
	}
	pair, ok := resultMap["BCHXBT"]
	if !ok {
		return errors.New("KrakenDecoder: field `BCHXBT` not found")
	}
	pairMap, ok := pair.(map[string]interface{})
	if !ok {
		return errors.New("KrackenDecoder type assertion failure")
	}
	c, ok := pairMap["c"]
	if !ok {
		return errors.New("KrakenDecoder: field `c` not found")
	}
	cList, ok := c.([]interface{})
	if !ok {
		return errors.New("KrackenDecoder type assertion failure")
	}
	rateStr, ok := cList[0].(string)
	if !ok {
		return errors.New("KrackenDecoder type assertion failure")
	}
	price, err := strconv.ParseFloat(rateStr, 64)
	if err != nil {
		return err
	}
	rate := price

	if rate == 0 {
		return errors.New("Bitcoin-ZCash price data not available")
	}
	for k, v := range rates {
		cache[k] = v * rate
	}
	return nil
}

func (b BitfinexDecoder) decode(dat interface{}, cache map[string]float64, bp *exchange.BitcoinPriceFetcher) (err error) {
	rates, err := bp.GetAllRates(false)
	if err != nil {
		return err
	}
	obj, ok := dat.(map[string]interface{})
	if !ok {
		return errors.New("BitfinexDecoder type assertion failure")
	}
	r, ok := obj["last_price"]
	if !ok {
		return errors.New("BitfinexDecoder: field `last_price` not found")
	}
	rateStr, ok := r.(string)
	if !ok {
		return errors.New("BitfinexDecoder type assertion failure")
	}
	price, err := strconv.ParseFloat(rateStr, 64)
	if err != nil {
		return err
	}
	rate := price

	if rate == 0 {
		return errors.New("Bitcoin-ZCash price data not available")
	}
	for k, v := range rates {
		cache[k] = v * rate
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
		return errors.New("Bitcoin-ZCash price data not available")
	}
	for k, v := range rates {
		cache[k] = v * rate
	}
	return nil
}

func (b PoloniexDecoder) decode(dat interface{}, cache map[string]float64, bp *exchange.BitcoinPriceFetcher) (err error) {
	rates, err := bp.GetAllRates(false)
	if err != nil {
		return err
	}
	data, ok := dat.(map[string]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed")
	}
	var rate float64
	v, ok := data["BTC_ZEC"]
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed")
	}
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
	if rate == 0 {
		return errors.New("Bitcoin-Zcash price data not available")
	}
	for k, v := range rates {
		cache[k] = v * rate
	}
	return nil
}
