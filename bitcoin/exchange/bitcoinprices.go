package exchange

import (
	"encoding/json"
	"errors"
	"github.com/op/go-logging"
	"golang.org/x/net/proxy"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const SatoshiPerBTC = 100000000

var log = logging.MustGetLogger("exchangeRates")

type ExchangeRateProvider interface {
	fetch() error
}

type BitcoinPriceFetcher struct {
	sync.Mutex
	cache     map[string]float64
	providers []ExchangeRateProvider
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
	b.providers = []ExchangeRateProvider{&BitcoinAverage{b.cache, client}, &BitPay{b.cache, client}, &BlockchainInfo{b.cache, client}, &BitcoinCharts{b.cache, client}}

	go b.run()
	return &b
}

func (b *BitcoinPriceFetcher) GetExchangeRate(currencyCode string) (float64, error) {
	b.Lock()
	defer b.Unlock()
	price, ok := b.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (b *BitcoinPriceFetcher) GetLatestRate(currencyCode string) (float64, error) {
	b.fetchCurrentRates()
	b.Lock()
	defer b.Unlock()
	price, ok := b.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (b *BitcoinPriceFetcher) GetAllRates() (map[string]float64, error) {
	b.Lock()
	defer b.Unlock()
	return b.cache, nil
}

func (b *BitcoinPriceFetcher) UnitsPerCoin() int {
	return SatoshiPerBTC
}

func (b *BitcoinPriceFetcher) run() {
	b.fetchCurrentRates()
	ticker := time.NewTicker(time.Minute * 15)
	for range ticker.C {
		b.fetchCurrentRates()
	}
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

type BitcoinAverage struct {
	cache  map[string]float64
	client *http.Client
}

func (b *BitcoinAverage) fetch() (err error) {
	resp, err := b.client.Get("https://ticker.openbazaar.org/api")
	if err != nil {
		return err
	}
	return b.decode(resp.Body)
}

func (b *BitcoinAverage) decode(body io.ReadCloser) (err error) {
	decoder := json.NewDecoder(body)
	var data map[string]interface{}
	err = decoder.Decode(&data)
	if err != nil {
		return err
	}
	for k, v := range data {
		if k != "timestamp" {
			val, ok := v.(map[string]interface{})
			if !ok {
				return errors.New("Type assertion failed")
			}
			price, ok := val["last"].(float64)
			if !ok {
				return errors.New("Type assertion failed")
			}
			b.cache[k] = price
		}
	}
	return nil
}

type BitPay struct {
	cache  map[string]float64
	client *http.Client
}

func (b *BitPay) fetch() (err error) {
	resp, err := b.client.Get("https://bitpay.com/api/rates")
	if err != nil {
		return err
	}
	return b.decode(resp.Body)
}

func (b *BitPay) decode(body io.ReadCloser) (err error) {
	decoder := json.NewDecoder(body)
	var data []map[string]interface{}
	err = decoder.Decode(&data)
	if err != nil {
		return err
	}
	for _, obj := range data {
		k, ok := obj["code"].(string)
		if !ok {
			return errors.New("Type assertion failed")
		}
		price, ok := obj["rate"].(float64)
		if !ok {
			return errors.New("Type assertion failed")
		}
		b.cache[k] = price
	}
	return nil
}

type BlockchainInfo struct {
	cache  map[string]float64
	client *http.Client
}

func (b *BlockchainInfo) fetch() (err error) {
	resp, err := b.client.Get("https://blockchain.info/ticker")
	if err != nil {
		return err
	}
	return b.decode(resp.Body)
}

func (b *BlockchainInfo) decode(body io.ReadCloser) (err error) {
	decoder := json.NewDecoder(body)
	var data map[string]interface{}
	err = decoder.Decode(&data)
	if err != nil {
		return err
	}
	for k, v := range data {
		val, ok := v.(map[string]interface{})
		if !ok {
			return errors.New("Type assertion failed")
		}
		price, ok := val["last"].(float64)
		if !ok {
			return errors.New("Type assertion failed")
		}
		b.cache[k] = price
	}
	return nil
}

type BitcoinCharts struct {
	cache  map[string]float64
	client *http.Client
}

func (b *BitcoinCharts) fetch() (err error) {
	resp, err := b.client.Get("https://api.bitcoincharts.com/v1/weighted_prices.json")
	if err != nil {
		return err
	}
	return b.decode(resp.Body)
}

func (b *BitcoinCharts) decode(body io.ReadCloser) (err error) {
	decoder := json.NewDecoder(body)
	var data map[string]interface{}
	err = decoder.Decode(&data)
	if err != nil {
		return err
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
			b.cache[k] = price
		}
	}
	return nil
}
