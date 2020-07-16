package client_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/OpenBazaar/multiwallet/client"
	"github.com/OpenBazaar/multiwallet/model"
	"github.com/OpenBazaar/multiwallet/model/mock"
	"github.com/OpenBazaar/multiwallet/test/factory"
	"github.com/jarcoal/httpmock"
)

func replaceHTTPClientOnClientPool(p *client.ClientPool, c http.Client) {
	for _, cp := range p.Clients() {
		cp.HTTPClient = c
	}
	p.HTTPClient = c
}

func mustPrepareClientPool(endpoints []string) (*client.ClientPool, func()) {
	var p, err = client.NewClientPool(endpoints, nil)
	if err != nil {
		panic(err.Error())
	}

	mockedHTTPClient := http.Client{}
	httpmock.ActivateNonDefault(&mockedHTTPClient)
	replaceHTTPClientOnClientPool(p, mockedHTTPClient)

	mock.MockWebsocketClientOnClientPool(p)
	err = p.Start()
	if err != nil {
		panic(err.Error())
	}

	return p, func() {
		httpmock.DeactivateAndReset()
		p.Close()
	}
}

func TestRequestRotatesServersOn500(t *testing.T) {
	var (
		endpointOne = "http://localhost:8332"
		endpointTwo = "http://localhost:8336"
		p, cleanup  = mustPrepareClientPool([]string{endpointOne, endpointTwo})
		expectedTx  = factory.NewTransaction()
		txid        = "1be612e4f2b79af279e0b307337924072b819b3aca09fcb20370dd9492b83428"
	)
	defer cleanup()

	httpmock.RegisterResponder(http.MethodGet, fmt.Sprintf("%s/tx/%s", endpointOne, txid),
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(http.StatusInternalServerError, expectedTx)
		},
	)
	httpmock.RegisterResponder(http.MethodGet, fmt.Sprintf("%s/tx/%s", endpointTwo, txid),
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(http.StatusOK, expectedTx)
		},
	)

	_, err := p.GetTransaction(txid)
	if err != nil {
		t.Errorf("expected successful transaction, but got error: %s", err.Error())
	}
}

func TestRequestRetriesTimeoutsToExhaustionThenRotates(t *testing.T) {
	var (
		endpointOne       = "http://localhost:8332"
		endpointTwo       = "http://localhost:8336"
		fastTimeoutClient = http.Client{Timeout: 500000 * time.Nanosecond}
		p, err            = client.NewClientPool([]string{endpointOne, endpointTwo}, nil)
	)
	if err != nil {
		t.Fatal(err)
	}

	httpmock.DeactivateAndReset()
	httpmock.ActivateNonDefault(&fastTimeoutClient)
	replaceHTTPClientOnClientPool(p, fastTimeoutClient)
	mock.MockWebsocketClientOnClientPool(p)
	if err = p.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		httpmock.DeactivateAndReset()
		p.Close()
	}()

	var (
		txid             = "1be612e4f2b79af279e0b307337924072b819b3aca09fcb20370dd9492b83428"
		expectedAttempts = uint(3)
		requestAttempts  uint
		laggyResponse    = func(req *http.Request) (*http.Response, error) {
			if requestAttempts < expectedAttempts {
				requestAttempts++
				time.Sleep(1 * time.Second)
				return nil, fmt.Errorf("timeout")
			}
			return httpmock.NewJsonResponse(http.StatusOK, factory.NewTransaction())
		}
	)
	httpmock.RegisterResponder(http.MethodGet, fmt.Sprintf("%s/tx/%s", endpointOne, txid), laggyResponse)
	httpmock.RegisterResponder(http.MethodGet, fmt.Sprintf("%s/tx/%s", endpointTwo, txid), laggyResponse)

	_, err = p.GetTransaction(txid)
	if err == nil {
		t.Errorf("expected getTransaction to respond with timeout error, but did not")
		return
	}
	if requestAttempts != expectedAttempts {
		t.Errorf("expected initial server to be attempted %d times, but was attempted only %d", expectedAttempts, requestAttempts)
	}
	_, err = p.GetTransaction(txid)
	if err != nil {
		t.Errorf("expected getTransaction to rotate to the next server and succeed, but returned error: %s", err.Error())
	}
}

func TestPoolBlockNotifyWorksAfterRotation(t *testing.T) {
	var (
		endpointOne = "http://localhost:8332"
		endpointTwo = "http://localhost:8336"
		testHash    = "0000000000000000003f1fb88ac3dab0e607e87def0e9031f7bea02cb464a04f"
		txid        = "1be612e4f2b79af279e0b307337924072b819b3aca09fcb20370dd9492b83428"
		testPath    = func(host string) string { return fmt.Sprintf("%s/tx/%s", host, txid) }
		p, cleanup  = mustPrepareClientPool([]string{endpointOne, endpointTwo})
	)
	defer cleanup()

	// GetTransaction should fail for endpoint one and succeed for endpoint two
	var (
		beenBad     bool
		badThenGood = func(req *http.Request) (*http.Response, error) {
			if beenBad {
				return httpmock.NewJsonResponse(http.StatusOK, factory.NewTransaction())
			}
			beenBad = true
			return httpmock.NewJsonResponse(http.StatusInternalServerError, nil)
		}
	)
	httpmock.RegisterResponder(http.MethodGet, testPath(endpointOne), badThenGood)
	httpmock.RegisterResponder(http.MethodGet, testPath(endpointTwo), badThenGood)

	go func() {
		c := p.PoolManager().AcquireCurrentWhenReady()
		c.BlockChannel() <- model.Block{Hash: testHash}
		p.PoolManager().ReleaseCurrent()
	}()

	ticker := time.NewTicker(time.Second * 2)
	select {
	case <-ticker.C:
		t.Error("Timed out waiting for block")
	case b := <-p.BlockNotify():
		if b.Hash != testHash {
			t.Error("Returned incorrect block hash")
		}
	}
	ticker.Stop()

	// request transaction triggers rotation
	if _, err := p.GetTransaction(txid); err != nil {
		t.Fatal(err)
	}

	go func() {
		c := p.PoolManager().AcquireCurrentWhenReady()
		c.BlockChannel() <- model.Block{Hash: testHash}
		p.PoolManager().ReleaseCurrent()
	}()

	ticker = time.NewTicker(time.Second * 2)
	select {
	case <-ticker.C:
		t.Error("Timed out waiting for block")
	case b := <-p.BlockNotify():
		if b.Hash != testHash {
			t.Error("Returned incorrect block hash")
		}
	}
	ticker.Stop()
}

func TestTransactionNotifyWorksAfterRotation(t *testing.T) {
	var (
		endpointOne  = "http://localhost:8332"
		endpointTwo  = "http://localhost:8336"
		expectedTx   = factory.NewTransaction()
		expectedTxid = "500000e4f2b79af279e0b307337924072b819b3aca09fcb20370dd9492b83428"
		testPath     = func(host string) string { return fmt.Sprintf("%s/tx/%s", host, expectedTxid) }
		p, cleanup   = mustPrepareClientPool([]string{endpointOne, endpointTwo})
	)
	defer cleanup()
	expectedTx.Txid = expectedTxid

	// GetTransaction should fail for endpoint one and succeed for endpoint two
	var (
		beenBad     bool
		badThenGood = func(req *http.Request) (*http.Response, error) {
			if beenBad {
				return httpmock.NewJsonResponse(http.StatusOK, expectedTx)
			}
			beenBad = true
			return httpmock.NewJsonResponse(http.StatusInternalServerError, nil)
		}
	)
	httpmock.RegisterResponder(http.MethodGet, testPath(endpointOne), badThenGood)
	httpmock.RegisterResponder(http.MethodGet, testPath(endpointTwo), badThenGood)

	go func() {
		c := p.PoolManager().AcquireCurrentWhenReady()
		c.TxChannel() <- expectedTx
		p.PoolManager().ReleaseCurrent()
	}()

	ticker := time.NewTicker(time.Second * 2)
	select {
	case <-ticker.C:
		t.Error("Timed out waiting for tx")
	case b := <-p.TransactionNotify():
		if b.Txid != expectedTx.Txid {
			t.Error("Returned incorrect tx hash")
		}
	}
	ticker.Stop()

	// request transaction triggers rotation
	if _, err := p.GetTransaction(expectedTxid); err != nil {
		t.Fatal(err)
	}

	go func() {
		c := p.PoolManager().AcquireCurrentWhenReady()
		c.TxChannel() <- expectedTx
		p.PoolManager().ReleaseCurrent()
	}()

	ticker = time.NewTicker(time.Second * 2)
	select {
	case <-ticker.C:
		t.Error("Timed out waiting for tx")
	case b := <-p.TransactionNotify():
		if b.Txid != expectedTx.Txid {
			t.Error("Returned incorrect tx hash")
		}
	}
	ticker.Stop()
}
