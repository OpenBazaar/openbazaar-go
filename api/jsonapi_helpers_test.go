package api

import (
	"bytes"
	"encoding/json"
	ma "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	"gx/ipfs/Qmc85NSvmSG4Frn9Vb2cBc1rMyULH6D3TNVEfCzSKoUpip/go-multiaddr-net"

	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/test"
	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
)

// testURIRoot is the root http URI to hit for testing
const testURIRoot = "http://127.0.0.1:9191"

// anyResponseJSON is a sentinel denoting any valid JSON response body is valid
const anyResponseJSON = "__anyresponsebodyJSON__"

// testHTTPClient is the http client to use for tests
var testHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// newTestGateway starts a new API gateway listening on the default test interface
func newTestGateway() (*Gateway, error) {
	// Create a test node, cookie, and config
	node, err := test.NewNode()
	if err != nil {
		return nil, err
	}

	apiConfig, err := test.NewAPIConfig()
	if err != nil {
		return nil, err
	}

	// Create an address to bind the API to
	addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/9191")
	if err != nil {
		return nil, err
	}

	listener, err := manet.Listen(addr)
	if err != nil {
		return nil, err
	}

	return NewGateway(node, *test.GetAuthCookie(), manet.NetListener(listener), *apiConfig, logging.NewLogBackend(os.Stdout, "", 0))
}

// apiTest is a test case to be run against the api blackbox
type apiTest struct {
	method, path string
	requestBody  interface{}

	expectedResponseCode int
	expectedResponseBody interface{}
}

// setupAction is used to change state before and after a set of []apiTest
type setupAction func(*test.Repository) error

// apiTests is a slice of apiTest
type apiTests []apiTest

func runAPITests(t *testing.T, tests apiTests) {
	_, err := test.ResetRepository()
	if err != nil {
		t.Fatal(err)
	}

	for _, jsonAPITest := range tests {
		executeAPITest(t, jsonAPITest)
	}
}

func runAPITestsWithSetup(t *testing.T, tests apiTests, runBefore, runAfter setupAction) {
	repository, err := test.ResetRepository()
	if err != nil {
		t.Fatal(err)
	}

	if runBefore != nil {
		if err := runBefore(repository); err != nil {
			t.Fatal("runBefore:", err)
		}
	}

	for _, jsonAPITest := range tests {
		executeAPITest(t, jsonAPITest)
	}

	if runAfter != nil {
		if err := runAfter(repository); err != nil {
			t.Fatal("runAfter:", err)
		}
	}
}

func runAPITest(t *testing.T, subject apiTest) {
	_, err := test.ResetRepository()
	if err != nil {
		t.Fatal(err)
	}
	executeAPITest(t, subject)
}

// executeAPITest executes the given test against the blackbox
func executeAPITest(t *testing.T, test apiTest) {
	var reqBody, expectedResp string
	if r, ok := test.requestBody.(string); ok {
		reqBody = r
	} else {
		mr, err := json.MarshalIndent(test.requestBody, "", "    ")
		if err != nil {
			t.Fatalf("marshalling requestBody: %s", err)
		}
		reqBody = string(mr)
	}
	if r, ok := test.expectedResponseBody.(string); ok {
		expectedResp = r
	} else {
		mr, err := json.MarshalIndent(test.expectedResponseBody, "", "    ")
		if err != nil {
			t.Fatalf("marshalling expectedResponseBody: %s", err)
		}
		expectedResp = string(mr)
	}

	t.Logf("request body:\n%s", reqBody)
	// Make the request
	testRequest, err := buildRequest(test.method, test.path, reqBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := testHTTPClient.Do(testRequest)
	if err != nil {
		t.Fatal(err)
	}
	// Ensure correct status code
	if resp.StatusCode != test.expectedResponseCode {
		b, _ := ioutil.ReadAll(resp.Body)
		t.Error(test.method, test.path, string(b))
		t.Errorf("Wanted status %d, got %d", test.expectedResponseCode, resp.StatusCode)
		return
	}

	// Read response body
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Parse response as JSON
	var responseJSON interface{}
	err = json.Unmarshal(respBody, &responseJSON)
	if err != nil {
		t.Fatal(err)
	}

	// Unless explicitly saying any JSON is expected check for equality
	if expectedResp != anyResponseJSON {
		var expectedJSON interface{}
		err = json.Unmarshal([]byte(expectedResp), &expectedJSON)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(responseJSON, expectedJSON) {
			t.Error("Error: incorrect response")
			t.Logf("expected:\n%s", expectedResp)
			t.Logf("actual:\n%s", string(respBody))
		}
	}
}

// buildRequest issues an http request directly to the blackbox handler
func buildRequest(method string, path string, body string) (*http.Request, error) {
	// Create a JSON request to the given endpoint
	req, err := http.NewRequest(method, testURIRoot+path, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}

	// Set headers/auth/cookie
	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth("test", "test")
	req.AddCookie(test.GetAuthCookie())

	return req, nil
}

func errorResponseJSON(err error) string {
	return `{"success": false, "reason": "` + err.Error() + `"}`
}

func httpGet(endpoint string) ([]byte, error) {
	req, err := buildRequest("GET", endpoint, "")
	if err != nil {
		return nil, err
	}
	resp, err := testHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, err
	}
	return ioutil.ReadAll(resp.Body)
}

func jsonFor(t *testing.T, fixture proto.Message) string {
	m := jsonpb.Marshaler{}

	jsonStr, err := m.MarshalToString(fixture)
	if err != nil {
		t.Fatal(err)
	}
	return jsonStr
}
