package litecoin

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"

	exchange "github.com/OpenBazaar/spvwallet/exchangerates"
	"golang.org/x/net/proxy"
	"strings"
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

type LitecoinPriceFetcher struct {
	sync.Mutex
	cache     map[string]float64
	providers []*ExchangeRateProvider
}

func NewLitecoinPriceFetcher(dialer proxy.Dialer) *LitecoinPriceFetcher {
	bp := exchange.NewBitcoinPriceFetcher(dialer)
	z := LitecoinPriceFetcher{
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
		{"https://bittrex.com/api/v1.1/public/getticker?market=btc-ltc", z.cache, client, BittrexDecoder{}, bp},
		{"https://api.bitfinex.com/v1/pubticker/ltcbtc", z.cache, client, BitfinexDecoder{}, bp},
		{"https://poloniex.com/public?command=returnTicker", z.cache, client, PoloniexDecoder{}, bp},
		{"https://api.kraken.com/0/public/Ticker?pair=LTCXBT", z.cache, client, KrakenDecoder{}, bp},
	}
	go z.run()
	return &z
}

func (z *LitecoinPriceFetcher) GetExchangeRate(currencyCode string) (float64, error) {
	currencyCode = NormalizeCurrencyCode(currencyCode)

	z.Lock()
	defer z.Unlock()
	price, ok := z.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (z *LitecoinPriceFetcher) GetLatestRate(currencyCode string) (float64, error) {
	currencyCode = NormalizeCurrencyCode(currencyCode)

	z.fetchCurrentRates()
	z.Lock()
	defer z.Unlock()
	price, ok := z.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (z *LitecoinPriceFetcher) GetAllRates(cacheOK bool) (map[string]float64, error) {
	if !cacheOK {
		err := z.fetchCurrentRates()
		if err != nil {
			return nil, err
		}
	}
	z.Lock()
	defer z.Unlock()
	copy := make(map[string]float64, len(z.cache))
	for k, v := range z.cache {
		copy[k] = v
	}
	return copy, nil
}

func (z *LitecoinPriceFetcher) UnitsPerCoin() int {
	return exchange.SatoshiPerBTC
}

func (z *LitecoinPriceFetcher) fetchCurrentRates() error {
	z.Lock()
	defer z.Unlock()
	for _, provider := range z.providers {
		err := provider.fetch()
		if err == nil {
			return nil
		}
	}
	return errors.New("all exchange rate API queries failed")
}

func (z *LitecoinPriceFetcher) run() {
	z.fetchCurrentRates()
	ticker := time.NewTicker(time.Minute * 15)
	for range ticker.C {
		z.fetchCurrentRates()
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
	return provider.decoder.decode(dataMap, provider.cache, provider.bitcoinProvider)
}

func (b OpenBazaarDecoder) decode(dat interface{}, cache map[string]float64, bp *exchange.BitcoinPriceFetcher) (err error) {
	data, ok := dat.(map[string]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed invalid json")
	}

	ltc, ok := data["LTC"]
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed, missing 'ZEC' field")
	}
	val, ok := ltc.(map[string]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: Type assertion failed")
	}
	ltcRate, ok := val["last"].(float64)
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
			cache[k] = price * (1 / ltcRate)
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
	pair, ok := resultMap["XLTCXXBT"]
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
		return errors.New("Bitcoin-litecoin price data not available")
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
		return errors.New("Bitcoin-litecoin price data not available")
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
		return errors.New("Bitcoin-litecoin price data not available")
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
	v := data["BTC_LTC"]
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
		return errors.New("Bitcoin-litecoin price data not available")
	}
	for k, v := range rates {
		cache[k] = v * rate
	}
	return nil
}

// NormalizeCurrencyCode standardizes the format for the given currency code
func NormalizeCurrencyCode(currencyCode string) string {
	return strings.ToUpper(currencyCode)
}
