package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/test"
	"github.com/stretchr/testify/assert"
)

// anyResponseJSON is a sentinel denoting any valid JSON response body is valid
const anyResponseJSON = "__anyresponsebodyJSON__"

// runJSONAPIBlackboxTests creates a blackbox and executes all given tests against it
func runJSONAPIBlackboxTests(t *testing.T, tests jsonAPIBlackboxTests) {
	bb := newJSONAPIBlackbox(t)
	for _, test := range tests {
		bb.runTest(test)
	}
}

// jsonAPIBlackboxTest is a test case to be run against the api blackbox
type jsonAPIBlackboxTest struct {
	method      string
	path        string
	requestBody string

	expectedResponseCode int
	expectedResponseBody string
}

// jsonAPIBlackboxTests is a slice of jsonAPIBlackboxTest
type jsonAPIBlackboxTests []jsonAPIBlackboxTest

// jsonAPIBlackbox is a testing oracle for the JSON API
type jsonAPIBlackbox struct {
	t           *testing.T
	handlerFunc http.HandlerFunc
}

// newJSONAPIBlackbox creates a new testing blackbox with a mock node
func newJSONAPIBlackbox(t *testing.T) jsonAPIBlackbox {
	// Create a test node, cookie, and config
	node := test.NewNode(t)
	var authCookie http.Cookie
	apiConfig, err := test.NewAPIConfig()
	if err != nil {
		t.Fatal(err)
	}

	// Create a new jsonAPIHandler to test
	apiHandler, err := newJsonAPIHandler(node, authCookie, *apiConfig)
	if err != nil {
		t.Fatal(err)
	}

	// Return a new blackbox
	return jsonAPIBlackbox{
		t:           t,
		handlerFunc: apiHandler.ServeHTTP,
	}
}

// request issues an http request directly to the blackbox handler
func (bb *jsonAPIBlackbox) request(method string, path string, body string) *http.Response {
	// Create a JSON request to the given endpoint
	req, err := http.NewRequest(method, path, bytes.NewBufferString(body))
	if err != nil {
		bb.t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/json")

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Execute the call and return the result and the response body
	bb.handlerFunc(rr, req)

	return rr.Result()
}

// runTest executes the given test against the blackbox
func (bb *jsonAPIBlackbox) runTest(test jsonAPIBlackboxTest) {
	// Make the request
	resp := bb.request(test.method, test.path, test.requestBody)

	// Ensure correct status code
	isEqual := assert.Equal(bb.t, test.expectedResponseCode, resp.StatusCode)
	if !isEqual {
		return
	}

	// Parse response as json
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		bb.t.Fatal(err)
	}

	var responseJSON interface{}
	err = json.Unmarshal(respBody, &responseJSON)
	if err != nil {
		bb.t.Fatal(err)
	}

	// Unless explicity saying any JSON is expected check for equality
	if test.expectedResponseBody != anyResponseJSON {
		var expectedJSON interface{}
		err = json.Unmarshal([]byte(test.expectedResponseBody), &expectedJSON)
		if err != nil {
			bb.t.Fatal(err)
		}

		isEqual = assert.True(bb.t, reflect.DeepEqual(responseJSON, expectedJSON))
		if !isEqual {
			fmt.Println("expected:", test.expectedResponseBody)
			fmt.Println("actual:", string(respBody))
		}
	}
}
