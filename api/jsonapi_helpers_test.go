package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/test"
	"github.com/golang/protobuf/proto"

	manet "gx/ipfs/QmX3U3YXCQ6UYBxq2LVWF8dARS1hPUTEYLrSx654Qyxyw6/go-multiaddr-net"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	"os"

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

	return NewGateway(node, *test.GetAuthCookie(), listener.NetListener(), *apiConfig, logging.NewLogBackend(os.Stdout, "", 0))
}

// apiTest is a test case to be run against the api blackbox
type apiTest struct {
	method      string
	path        string
	requestBody string

	expectedResponseCode int
	expectedResponseBody string
}

// apiTests is a slice of apiTest
type apiTests []apiTest

// request issues an http request directly to the blackbox handler
func request(req *http.Request) (*http.Response, error) {
	resp, err := testHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func resetTestRepo(t *testing.T) {
	repository, err := test.NewRepository()
	if err != nil {
		t.Fatal(err)
	}

	repository.Reset()
	if err != nil {
		t.Fatal(err)
	}
}

func runAPITests(t *testing.T, tests apiTests) {
	resetTestRepo(t)

	// Run each test in serial
	for _, jsonAPITest := range tests {
		executeAPITest(t, jsonAPITest)
	}
}

func runAPITest(t *testing.T, test apiTest) {
	resetTestRepo(t)
	executeAPITest(t, test)
}

// executeAPITest executes the given test against the blackbox
func executeAPITest(t *testing.T, test apiTest) {
	// Make the request
	req, err := buildRequest(test.method, test.path, test.requestBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := testHTTPClient.Do(req)
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
	if test.expectedResponseBody != anyResponseJSON {
		var expectedJSON interface{}
		err = json.Unmarshal([]byte(test.expectedResponseBody), &expectedJSON)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(responseJSON, expectedJSON) {
			fmt.Println("expected:", test.expectedResponseBody)
			fmt.Println("actual:", string(respBody))
			t.Fatal("Incorrect response")
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

func jsonFor(t *testing.T, fixture proto.Message) string {
	m := jsonpb.Marshaler{}

	json, err := m.MarshalToString(fixture)
	if err != nil {
		t.Fatal(err)
	}
	return json
}
