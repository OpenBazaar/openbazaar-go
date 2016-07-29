package exchange

import (
	"encoding/json"
	"errors"
	"github.com/op/go-logging"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var log = logging.MustGetLogger("ipfs")

type BitcoinPriceFetcher struct {
	sync.Mutex
	cache map[string]float64
}

func NewBitcoinPriceFetcher() *BitcoinPriceFetcher {
	b := BitcoinPriceFetcher{
		cache: make(map[string]float64),
	}
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

func (b *BitcoinPriceFetcher) run() {
	b.fetchCurrentRates()
	ticker := time.NewTicker(time.Minute * 15)
	for range ticker.C {
		b.fetchCurrentRates()
	}
}

func (b *BitcoinPriceFetcher) fetchCurrentRates() {
	b.Lock()
	defer b.Unlock()
	log.Infof("Fetching bitcoin exchange rates")
	err := b.fetchBitcoinAverage()
	if err == nil {
		return
	}
	err = b.fetchBitpay()
	if err == nil {
		return
	}
	err = b.fetchBlockchainDotInfo()
	if err == nil {
		return
	}
	err = b.fetchBitcoinCharts()
	if err != nil {
		log.Error("Failed to fetch bitcoin exchange rates")
	}

}

func (b *BitcoinPriceFetcher) fetchBitcoinAverage() (err error) {
	defer func() {
		if rerr := recover(); rerr != nil {
			err = errors.New("Panic fetching exchange rates")
		}
	}()
	resp, err := http.Get("https://api.bitcoinaverage.com/ticker/global/all")
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(resp.Body)
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

func (b *BitcoinPriceFetcher) fetchBitpay() (err error) {
	defer func() {
		if rerr := recover(); rerr != nil {
			err = errors.New("Panic fetching exchange rates")
		}
	}()
	resp, err := http.Get("https://bitpay.com/api/rates")
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(resp.Body)
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

func (b *BitcoinPriceFetcher) fetchBlockchainDotInfo() (err error) {
	defer func() {
		if rerr := recover(); rerr != nil {
			err = errors.New("Panic fetching exchange rates")
		}
	}()
	resp, err := http.Get("https://blockchain.info/ticker")
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(resp.Body)
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

func (b *BitcoinPriceFetcher) fetchBitcoinCharts() (err error) {
	defer func() {
		if rerr := recover(); rerr != nil {
			err = errors.New("Panic fetching exchange rates")
		}
	}()
	resp, err := http.Get("https://api.bitcoincharts.com/v1/weighted_prices.json")
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(resp.Body)
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
