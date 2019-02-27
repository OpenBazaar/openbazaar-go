package wallet

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

	exchange "github.com/OpenBazaar/spvwallet/exchangerates"
	"golang.org/x/net/proxy"
)

// ExchangeRateProvider - used for looking up exchange rates for ETH
type ExchangeRateProvider struct {
	fetchURL        string
	cache           map[string]float64
	client          *http.Client
	decoder         ExchangeRateDecoder
	bitcoinProvider *exchange.BitcoinPriceFetcher
}

// ExchangeRateDecoder - used for serializing/deserializing provider struct
type ExchangeRateDecoder interface {
	decode(dat interface{}, cache map[string]float64, bp *exchange.BitcoinPriceFetcher) (err error)
}

// OpenBazaarDecoder - decoder to be used by OB
type OpenBazaarDecoder struct{}

// KrakenDecoder - decoder with Kraken exchange as provider
type KrakenDecoder struct{}

// PoloniexDecoder - decoder with Poloniex exchange as provider
type PoloniexDecoder struct{}

// BitfinexDecoder - decoder with Bitfinex exchange as provider
type BitfinexDecoder struct{}

// BittrexDecoder - decoder with Bittrex exchange as provider
type BittrexDecoder struct{}

// EthereumPriceFetcher - get ETH prices from the providers (exchanges)
type EthereumPriceFetcher struct {
	sync.Mutex
	cache     map[string]float64
	providers []*ExchangeRateProvider
}

// NewEthereumPriceFetcher - instantiate a eth price fetcher
func NewEthereumPriceFetcher(dialer proxy.Dialer) *EthereumPriceFetcher {
	bp := exchange.NewBitcoinPriceFetcher(dialer)
	z := EthereumPriceFetcher{
		cache: make(map[string]float64),
	}
	dial := net.Dial
	if dialer != nil {
		dial = dialer.Dial
	}
	tbTransport := &http.Transport{Dial: dial}
	client := &http.Client{Transport: tbTransport, Timeout: time.Minute}

	z.providers = []*ExchangeRateProvider{
		{"https://api.kraken.com/0/public/Ticker?pair=ETHXBT", z.cache, client, KrakenDecoder{}, bp},
	}
	go z.run()
	return &z
}

// GetExchangeRate - fetch the exchange rate for the specified currency
func (z *EthereumPriceFetcher) GetExchangeRate(currencyCode string) (float64, error) {
	currencyCode = NormalizeCurrencyCode(currencyCode)

	z.Lock()
	defer z.Unlock()
	price, ok := z.cache[currencyCode]
	if !ok {
		return 0, errors.New("currency not tracked")
	}
	return price, nil
}

// GetLatestRate - refresh the cache and return the latest exchange rate for the specified currency
func (z *EthereumPriceFetcher) GetLatestRate(currencyCode string) (float64, error) {
	currencyCode = NormalizeCurrencyCode(currencyCode)

	z.fetchCurrentRates()
	z.Lock()
	defer z.Unlock()
	price, ok := z.cache[currencyCode]
	if !ok {
		return 0, errors.New("currency not tracked")
	}
	return price, nil
}

// GetAllRates - refresh the cache
func (z *EthereumPriceFetcher) GetAllRates(cacheOK bool) (map[string]float64, error) {
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

// UnitsPerCoin - return satoshis in 1 BTC
func (z *EthereumPriceFetcher) UnitsPerCoin() int {
	return exchange.SatoshiPerBTC
}

func (z *EthereumPriceFetcher) fetchCurrentRates() error {
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

func (z *EthereumPriceFetcher) run() {
	z.fetchCurrentRates()
	ticker := time.NewTicker(time.Minute * 15)
	for range ticker.C {
		z.fetchCurrentRates()
	}
}

func (provider *ExchangeRateProvider) fetch() (err error) {
	if len(provider.fetchURL) == 0 {
		err = errors.New("provider has no fetchUrl")
		return err
	}
	resp, err := provider.client.Get(provider.fetchURL)
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
	//data := dat.(map[string]interface{})
	data, ok := dat.(map[string]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: type assertion failed invalid json")
	}

	eth, ok := data["ETH"]
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: type assertion failed, missing 'ETH' field")
	}
	val, ok := eth.(map[string]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: type assertion failed")
	}
	ethRate, ok := val["last"].(float64)
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: type assertion failed, missing 'last' (float) field")
	}
	for k, v := range data {
		if k != "timestamp" {
			val, ok := v.(map[string]interface{})
			if !ok {
				return errors.New(reflect.TypeOf(b).Name() + ".decode: type assertion failed")
			}
			price, ok := val["last"].(float64)
			if !ok {
				return errors.New(reflect.TypeOf(b).Name() + ".decode: type assertion failed, missing 'last' (float) field")
			}
			cache[k] = price * (1 / ethRate)
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
		return errors.New("krakenDecoder type assertion failure")
	}
	result, ok := obj["result"]
	if !ok {
		return errors.New("krakenDecoder: field `result` not found")
	}
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return errors.New("KrakenDecoder type assertion failure")
	}
	pair, ok := resultMap["XETHXXBT"]
	if !ok {
		return errors.New("krakenDecoder: field `ETHXBT` not found")
	}
	pairMap, ok := pair.(map[string]interface{})
	if !ok {
		return errors.New("krakenDecoder type assertion failure")
	}
	c, ok := pairMap["c"]
	if !ok {
		return errors.New("krakenDecoder: field `c` not found")
	}
	cList, ok := c.([]interface{})
	if !ok {
		return errors.New("krakenDecoder type assertion failure")
	}
	rateStr, ok := cList[0].(string)
	if !ok {
		return errors.New("krakenDecoder type assertion failure")
	}
	price, err := strconv.ParseFloat(rateStr, 64)
	if err != nil {
		return err
	}
	rate := price

	if rate == 0 {
		return errors.New("bitcoin-ethereum price data not available")
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
		return errors.New("bitfinexDecoder: type assertion failure")
	}
	r, ok := obj["last_price"]
	if !ok {
		return errors.New("bitfinexDecoder: field `last_price` not found")
	}
	rateStr, ok := r.(string)
	if !ok {
		return errors.New("bitfinexDecoder: type assertion failure")
	}
	price, err := strconv.ParseFloat(rateStr, 64)
	if err != nil {
		return err
	}
	rate := price

	if rate == 0 {
		return errors.New("bitcoin-ethereum price data not available")
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
		return errors.New("bittrexDecoder: type assertion failure")
	}
	result, ok := obj["result"]
	if !ok {
		return errors.New("bittrexDecoder: field `result` not found")
	}
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return errors.New("bittrexDecoder: type assertion failure")
	}
	exRate, ok := resultMap["Last"]
	if !ok {
		return errors.New("bittrexDecoder: field `Last` not found")
	}
	rate, ok := exRate.(float64)
	if !ok {
		return errors.New("bittrexDecoder type assertion failure")
	}

	if rate == 0 {
		return errors.New("bitcoin-ethereum price data not available")
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
		return errors.New(reflect.TypeOf(b).Name() + ".decode: type assertion failed")
	}
	var rate float64
	v := data["BTC_ETH"]
	//data := dat.(map[string]interface{})
	//var rate float64

	val, ok := v.(map[string]interface{})
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: type assertion failed")
	}
	s, ok := val["last"].(string)
	if !ok {
		return errors.New(reflect.TypeOf(b).Name() + ".decode: type assertion failed, missing 'last' (string) field")
	}
	price, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	rate = price

	if rate == 0 {
		return errors.New("bitcoin-ethereum price data not available")
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
