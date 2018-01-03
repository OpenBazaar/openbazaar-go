package exchangerates

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/OpenBazaar/openbazaar-go/bitcoin/exchange"
	"golang.org/x/net/proxy"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
	"reflect"
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
type BitfinexDecoder struct{}
type BittrexDecoder struct{}
type PoloniexDecoder struct{}

type BitcoinCashPriceFetcher struct {
	sync.Mutex
	cache     map[string]float64
	providers []*ExchangeRateProvider
}

func NewBitcoinCashPriceFetcher(dialer proxy.Dialer) *BitcoinCashPriceFetcher {
	bp := exchange.NewBitcoinPriceFetcher(dialer)
	b := BitcoinCashPriceFetcher{
		cache: make(map[string]float64),
	}
	dial := net.Dial
	if dialer != nil {
		dial = dialer.Dial
	}
	tbTransport := &http.Transport{Dial: dial}
	client := &http.Client{Transport: tbTransport, Timeout: time.Minute}

	b.providers = []*ExchangeRateProvider{
		{"https://ticker.openbazaar.org/api", b.cache, client, OpenBazaarDecoder{}, nil},
		{"https://bittrex.com/api/v1.1/public/getticker?market=btc-bcc", b.cache, client, BittrexDecoder{}, bp},
		{"https://api.bitfinex.com/v1/pubticker/bchbtc", b.cache, client, BitfinexDecoder{}, bp},
		{"https://poloniex.com/public?command=returnTicker", b.cache, client, PoloniexDecoder{}, bp},
		{"https://api.kraken.com/0/public/Ticker?pair=BCHXBT", b.cache, client, KrakenDecoder{}, bp},
	}
	go b.run()
	return &b
}

func (b *BitcoinCashPriceFetcher) GetExchangeRate(currencyCode string) (float64, error) {
	b.Lock()
	defer b.Unlock()
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
	return exchange.SatoshiPerBTC
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

func (b *BitcoinCashPriceFetcher) run() {
	b.fetchCurrentRates()
	ticker := time.NewTicker(time.Minute * 15)
	for range ticker.C {
		b.fetchCurrentRates()
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
	data := dat.(map[string]interface{})

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
			cache[k] = price*(1/bchRate)
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
		return errors.New("Bitcoin-BitcoinCash price data not available")
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
		return errors.New("Bitcoin-BitcoinCash price data not available")
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
		return errors.New("Bitcoin-BitcoinCash price data not available")
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
	data := dat.(map[string]interface{})
	var rate float64
	for k, v := range data {
		if k == "BTC_BCH" {
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
		return errors.New("Bitcoin-BitcoinCash price data not available")
	}
	for k, v := range rates {
		cache[k] = v * rate
	}
	return nil
}
