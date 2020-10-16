package bitcoincash

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
)

func setupBitcoinPriceFetcher() (BitcoinCashPriceFetcher, func()) {
	var (
		url          = "https://ticker.openbazaar.org/api"
		mockResponse = `{
		"BCH": {
			"last": 20.00000,
			"type": "crypto"
		},
		"USD": {
			"last": 10000.00,
			"type": "fiat"
		}
	}`
		exchangeCache = make(map[string]float64)
	)

	httpmock.Activate()
	httpmock.RegisterResponder("GET", url,
		httpmock.NewStringResponder(200, mockResponse))

	return BitcoinCashPriceFetcher{
		cache:     exchangeCache,
		providers: []*ExchangeRateProvider{{url, exchangeCache, &http.Client{}, OpenBazaarDecoder{}}},
	}, httpmock.DeactivateAndReset
}

func TestFetchCurrentRates(t *testing.T) {
	b, teardown := setupBitcoinPriceFetcher()
	defer teardown()

	err := b.fetchCurrentRates()
	if err != nil {
		t.Error("Failed to fetch bitcoin exchange rates")
	}
}

func TestGetLatestRate(t *testing.T) {
	b, teardown := setupBitcoinPriceFetcher()
	defer teardown()

	price, ok := b.cache["USD"]
	if !ok && price == 500.00 {
		t.Errorf("incorrect cache value, expected (%f) but got (%f)", 500.00, price)
	}
	price, err := b.GetLatestRate("USD")
	if err != nil && price == 500.00 {
		t.Error("Incorrect return at GetLatestRate (price, err)", price, err)
	}
}

func TestGetAllRates(t *testing.T) {
	b, teardown := setupBitcoinPriceFetcher()
	defer teardown()

	b.cache["USD"] = 650.00
	b.cache["EUR"] = 600.00
	priceMap, err := b.GetAllRates(true)
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
	b, teardown := setupBitcoinPriceFetcher()
	defer teardown()

	b.cache["USD"] = 650.00
	r, err := b.GetExchangeRate("USD")
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

	// Test that currency symbols are normalized correctly
	r, err = b.GetExchangeRate("usd")
	if err != nil {
		t.Error("Failed to fetch exchange rate")
	}
	if r != 650.00 {
		t.Error("Returned exchange rate incorrect")
	}
}

type req struct {
	io.Reader
}

func (r *req) Close() error {
	return nil
}

func TestDecodeOpenBazaar(t *testing.T) {
	cache := make(map[string]float64)
	openbazaarDecoder := OpenBazaarDecoder{}
	var dataMap interface{}

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
	  "BCH": {
	    "ask":32.089016,
	    "bid":32.089016,
	    "last":32.089016,
	    "timestamp": "Tue, 02 Aug 2016 00:20:45 -0000"
	  },
	  "timestamp": "Tue, 02 Aug 2016 00:20:45 -0000"
	}`
	// Test valid response
	r := &req{bytes.NewReader([]byte(response))}
	decoder := json.NewDecoder(r)
	err := decoder.Decode(&dataMap)
	if err != nil {
		t.Error(err)
	}
	err = openbazaarDecoder.decode(dataMap, cache)
	if err != nil {
		t.Error(err)
	}
	// Make sure it saved to cache
	if len(cache) == 0 {
		t.Error("Failed to response to cache")
	}
	resp := `{"ZWL": {
	"ask": 196806.48,
	"bid": 196316.95,
	"timestamp": "Tue, 02 Aug 2016 00:20:45 -0000",
	"volume_btc": 0.0,
	"volume_percent": 0.0
	}}`

	// Test missing JSON element
	r = &req{bytes.NewReader([]byte(resp))}
	decoder = json.NewDecoder(r)
	err = decoder.Decode(&dataMap)
	if err != nil {
		t.Error(err)
	}
	err = openbazaarDecoder.decode(dataMap, cache)
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

	// Test invalid JSON
	r = &req{bytes.NewReader([]byte(resp))}
	decoder = json.NewDecoder(r)
	err = decoder.Decode(&dataMap)
	if err != nil {
		t.Error(err)
	}
	err = openbazaarDecoder.decode(dataMap, cache)
	if err == nil {
		t.Error(err)
	}

	// Test decode error
	r = &req{bytes.NewReader([]byte(""))}
	decoder = json.NewDecoder(r)
	decoder.Decode(&dataMap)
	err = openbazaarDecoder.decode(dataMap, cache)
	if err == nil {
		t.Error(err)
	}
}
