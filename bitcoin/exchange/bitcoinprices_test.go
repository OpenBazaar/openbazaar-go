package exchange

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type testExchangeRateProvider struct {
	err error
}

func (t *testExchangeRateProvider) fetch() error {
	return t.err
}

func TestFetchCurrentRates(t *testing.T) {
	b := BitcoinPriceFetcher{
		cache: make(map[string]float64),
	}
	b.providers = []ExchangeRateProvider{&testExchangeRateProvider{nil}, &testExchangeRateProvider{nil}, &testExchangeRateProvider{nil}}
	err := b.fetchCurrentRates()
	if err != nil {
		t.Error("Failed to fetch bitcoin exchange rates")
	}
	b.providers = []ExchangeRateProvider{&testExchangeRateProvider{errors.New("Query fail")}, &testExchangeRateProvider{errors.New("Query fail")}, &testExchangeRateProvider{errors.New("Query fail")}}
	err = b.fetchCurrentRates()
	if err == nil {
		t.Error("Failed to handle error when fetching exchange rates")
	}
}

func TestGetLastRate(t *testing.T) {
	b := BitcoinPriceFetcher{
		cache: make(map[string]float64),
	}
	b.providers = []ExchangeRateProvider{&testExchangeRateProvider{nil}, &testExchangeRateProvider{nil}, &testExchangeRateProvider{nil}}
	price, err := b.GetLatestRate("USD")
	if err == nil || price != 0 {
		t.Error("Incorrect return at GetLastRate")
	}
	b.cache["USD"] = 650.00
	price, err = b.GetLatestRate("USD")
	if err != nil || price != 650.00 {
		t.Error("Incorrect return at GetLastRate")
	}
}

func TestGetAllRates(t *testing.T) {
	b := BitcoinPriceFetcher{
		cache: make(map[string]float64),
	}
	b.providers = []ExchangeRateProvider{&testExchangeRateProvider{nil}, &testExchangeRateProvider{nil}, &testExchangeRateProvider{nil}}
	b.cache["USD"] = 650.00
	b.cache["EUR"] = 600.00
	priceMap, err := b.GetAllRates()
	if err != nil {
		t.Error(err)
	}
	usd, ok := priceMap["USD"]
	if !ok || usd != 650.00 {
		t.Error("Failed to fetch exchange rates from cache")
	}
	eur, ok := priceMap["EUR"]
	if !ok || eur != 600.00 {
		t.Error("Failed to fetch exchange rates from cache")
	}
}

func TestGetExchangeRate(t *testing.T) {
	b := BitcoinPriceFetcher{
		cache: make(map[string]float64),
	}
	b.cache["usd"] = 650.00
	r, err := b.GetExchangeRate("usd")
	if err != nil {
		t.Error("Failed to fetch exchange rate")
	}
	if r != 650.00 {
		t.Error("Returned exchange rate incorrect")
	}
	r, err = b.GetExchangeRate("EUR")
	if r != 0 || err == nil {
		t.Error("Return erroneous exchange rate")
	}
}

type req struct {
	io.Reader
}

func (r *req) Close() error {
	return nil
}

func TestDecodeBitcoinAverage(t *testing.T) {
	response := `{
	  "AED": {
	    "ask": 2242.19,
	    "bid": 2236.61,
	    "last": 2239.99,
	    "timestamp": "Tue, 02 Aug 2016 00:20:45 -0000",
	    "volume_btc": 0.0,
	    "volume_percent": 0.0
	  },
	  "AFN": {
	    "ask": 41849.95,
	    "bid": 41745.86,
	    "last": 41808.85,
	    "timestamp": "Tue, 02 Aug 2016 00:20:45 -0000",
	    "volume_btc": 0.0,
	    "volume_percent": 0.0
	  },
	  "ALL": {
	    "ask": 74758.44,
	    "bid": 74572.49,
	    "last": 74685.02,
	    "timestamp": "Tue, 02 Aug 2016 00:20:45 -0000",
	    "volume_btc": 0.0,
	    "volume_percent": 0.0
	  },
	  "timestamp": "Tue, 02 Aug 2016 00:20:45 -0000"
	}`
	// Test valid response
	r := &req{bytes.NewReader([]byte(response))}
	b := BitcoinAverage{
		cache: make(map[string]float64),
	}
	err := b.decode(r)
	if err != nil {
		t.Error(err)
	}
	// Make sure it saved to cache
	if len(b.cache) == 0 {
		t.Error("Failed to response to cache")
	}
	resp := `{"ZWL": {
	"ask": 196806.48,
	"bid": 196316.95,
	"timestamp": "Tue, 02 Aug 2016 00:20:45 -0000",
	"volume_btc": 0.0,
	"volume_percent": 0.0
	}}`

	// Test missing json element
	r = &req{bytes.NewReader([]byte(resp))}
	err = b.decode(r)
	if err == nil {
		t.Error(err)
	}
	resp = `{
	"ask": 196806.48,
	"bid": 196316.95,
	"last": 196613.2,
	"timestamp": "Tue, 02 Aug 2016 00:20:45 -0000",
	"volume_btc": 0.0,
	"volume_percent": 0.0
	}`

	// Test invalid json
	r = &req{bytes.NewReader([]byte(resp))}
	err = b.decode(r)
	if err == nil {
		t.Error(err)
	}

	// Test decode error
	r = &req{bytes.NewReader([]byte(""))}
	err = b.decode(r)
	if err == nil {
		t.Error(err)
	}
}

func TestDecodeBitPay(t *testing.T) {
	response := `[{"code":"BTC","name":"Bitcoin","rate":1},{"code":"USD","name":"US Dollar","rate":611.02},{"code":"EUR","name":"Eurozone Euro","rate":546.740696},{"code":"GBP","name":"Pound Sterling","rate":462.982074},{"code":"JPY","name":"Japanese Yen","rate":62479.23908}]`
	// Test valid response
	r := &req{bytes.NewReader([]byte(response))}
	b := BitPay{
		cache: make(map[string]float64),
	}
	err := b.decode(r)
	if err != nil {
		t.Error(err)
	}
	// Make sure it saved to cache
	if len(b.cache) == 0 {
		t.Error("Failed to response to cache")
	}

	resp := `[{"code":"BTC","name":"Bitcoin"},{"code":"USD","name":"US Dollar","rate":611.02},{"code":"EUR","name":"Eurozone Euro","rate":546.740696},{"code":"GBP","name":"Pound Sterling","rate":462.982074},{"code":"JPY","name":"Japanese Yen","rate":62479.23908}]`
	// Test missing json element
	r = &req{bytes.NewReader([]byte(resp))}
	err = b.decode(r)
	if err == nil {
		t.Error(err)
	}
	resp = `[{"name":"Bitcoin","rate":611.02},{"code":"USD","name":"US Dollar","rate":611.02},{"code":"EUR","name":"Eurozone Euro","rate":546.740696},{"code":"GBP","name":"Pound Sterling","rate":462.982074},{"code":"JPY","name":"Japanese Yen","rate":62479.23908}]`
	// Test missing json element
	r = &req{bytes.NewReader([]byte(resp))}
	err = b.decode(r)
	if err == nil {
		t.Error(err)
	}

	// Test decode error
	r = &req{bytes.NewReader([]byte(""))}
	err = b.decode(r)
	if err == nil {
		t.Error(err)
	}
}

func TestDecodeBlockChainInfo(t *testing.T) {
	response := `{"USD" : {"15m" : 612.73, "last" : 612.73, "buy" : 611.1, "sell" : 612.72,  "symbol" : "$"},
  "ISK" : {"15m" : 74706.49, "last" : 74706.49, "buy" : 74507.76, "sell" : 74705.27,  "symbol" : "kr"},
  "HKD" : {"15m" : 4752.76, "last" : 4752.76, "buy" : 4740.11, "sell" : 4752.68,  "symbol" : "$"}}`
	// Test valid response
	r := &req{bytes.NewReader([]byte(response))}
	b := BlockchainInfo{
		cache: make(map[string]float64),
	}
	err := b.decode(r)
	if err != nil {
		t.Error(err)
	}
	// Make sure it saved to cache
	if len(b.cache) == 0 {
		t.Error("Failed to response to cache")
	}

	resp := `{"USD" : [{"15m" : 612.73, "last" : 612.73, "buy" : 611.1, "sell" : 612.72,  "symbol" : "$"}],
  "ISK" : {"15m" : 74706.49, "last" : 74706.49, "buy" : 74507.76, "sell" : 74705.27,  "symbol" : "kr"},
  "HKD" : {"15m" : 4752.76, "last" : 4752.76, "buy" : 4740.11, "sell" : 4752.68,  "symbol" : "$"}}`
	// Test missing json element
	r = &req{bytes.NewReader([]byte(resp))}
	err = b.decode(r)
	if err == nil {
		t.Error(err)
	}
	resp = `{"USD" : {"15m" : 612.73, "buy" : 611.1, "sell" : 612.72,  "symbol" : "$"},
  "ISK" : {"15m" : 74706.49, "last" : 74706.49, "buy" : 74507.76, "sell" : 74705.27,  "symbol" : "kr"},
  "HKD" : {"15m" : 4752.76, "last" : 4752.76, "buy" : 4740.11, "sell" : 4752.68,  "symbol" : "$"}}`
	// Test missing json element
	r = &req{bytes.NewReader([]byte(resp))}
	err = b.decode(r)
	if err == nil {
		t.Error(err)
	}

	// Test decode error
	r = &req{bytes.NewReader([]byte(""))}
	err = b.decode(r)
	if err == nil {
		t.Error(err)
	}
}

func TestDecodeBitcoinCharts(t *testing.T) {
	response := `{"USD": {"7d": "642.47", "30d": "656.26", "24h": "618.68"}, "IDR": {"7d": "8473454.17", "30d": "8611783.41", "24h": "8118676.19"}, "ILS": {"7d": "2486.06", "30d": "2595.67", "24h": "2351.95"}, "GBP": {"7d": "499.01", "30d": "508.06", "24h": "479.65"}}`
	// Test valid response
	r := &req{bytes.NewReader([]byte(response))}
	b := BitcoinCharts{
		cache: make(map[string]float64),
	}
	err := b.decode(r)
	if err != nil {
		t.Error(err)
	}
	// Make sure it saved to cache
	if len(b.cache) == 0 {
		t.Error("Failed to response to cache")
	}

	resp := `{"USD": {"7d": "642.47", "30d": "656.26"}, "IDR": {"7d": "8473454.17", "30d": "8611783.41", "24h": "8118676.19"}, "ILS": {"7d": "2486.06", "30d": "2595.67", "24h": "2351.95"}, "GBP": {"7d": "499.01", "30d": "508.06", "24h": "479.65"}}`
	// Test missing json element
	r = &req{bytes.NewReader([]byte(resp))}
	err = b.decode(r)
	if err != nil {
		t.Error(err)
	}
	resp = `{"USD": {"7d": "642.47", "30d": "656.26", "24h": 618.68}, "IDR": {"7d": "8473454.17", "30d": "8611783.41", "24h": "8118676.19"}, "ILS": {"7d": "2486.06", "30d": "2595.67", "24h": "2351.95"}, "GBP": {"7d": "499.01", "30d": "508.06", "24h": "479.65"}}`
	// Test malformatted json
	r = &req{bytes.NewReader([]byte(resp))}
	err = b.decode(r)
	if err == nil {
		t.Error(err)
	}

	resp = `{"USD": {"7d": "642.47", "30d": "656.26", "24h": "asdf"}, "IDR": {"7d": "8473454.17", "30d": "8611783.41", "24h": "8118676.19"}, "ILS": {"7d": "2486.06", "30d": "2595.67", "24h": "2351.95"}, "GBP": {"7d": "499.01", "30d": "508.06", "24h": "479.65"}}`
	// Test malformatted json
	r = &req{bytes.NewReader([]byte(resp))}
	err = b.decode(r)
	if err == nil {
		t.Error(err)
	}

	resp = `{"USD": [{"7d": "642.47", "30d": "656.26", "24h": "615.00"}], "IDR": {"7d": "8473454.17", "30d": "8611783.41", "24h": "8118676.19"}, "ILS": {"7d": "2486.06", "30d": "2595.67", "24h": "2351.95"}, "GBP": {"7d": "499.01", "30d": "508.06", "24h": "479.65"}}`
	// Test malformatted json
	r = &req{bytes.NewReader([]byte(resp))}
	err = b.decode(r)
	if err == nil {
		t.Error(err)
	}

	// Test decode error
	r = &req{bytes.NewReader([]byte(""))}
	err = b.decode(r)
	if err == nil {
		t.Error(err)
	}
}
