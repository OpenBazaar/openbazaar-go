package exchange

import (
	"github.com/anacrolix/sync"
	"time"
	"encoding/json"
	"net/http"
	"errors"
	"github.com/op/go-logging"
	"strconv"
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
	price, ok := b.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (b *BitcoinPriceFetcher) GetLatestRate(currencyCode string) (float64, error) {
	b.fetchCurrentRates()
	price, ok := b.cache[currencyCode]
	if !ok {
		return 0, errors.New("Currency not tracked")
	}
	return price, nil
}

func (b *BitcoinPriceFetcher) run() {
	b.fetchCurrentRates()
	ticker := time.NewTicker(time.Minute * 15)
	for {
		select {
		case <- ticker.C:
			b.fetchCurrentRates()
		}
	}
}

func (b *BitcoinPriceFetcher) fetchCurrentRates() {
	log.Infof("Fetching bitcoin exchange rates...")
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
			price := v.(map[string]interface{})["last"].(float64)
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
		k := obj["code"].(string)
		price := obj["rate"].(float64)
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
		price := v.(map[string]interface{})["last"].(float64)
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
			p, ok := v.(map[string]interface{})["24h"]
			if !ok {
				continue
			}
			price, err := strconv.ParseFloat(p.(string), 64)
			if err != nil {
				return err
			}
			b.cache[k] = price
		}
	}
	return nil
}