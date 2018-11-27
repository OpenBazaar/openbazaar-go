package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIPrefixHandler(t *testing.T) {
	type testcase struct {
		prefix     string
		reqURL     string
		nextReqURL string
		respBody   string
		nextCalled bool
		status     int
	}

	tcs := []testcase{
		{
			prefix:     "/api/v0",
			reqURL:     "/api/v0/version",
			nextReqURL: "/version",
			respBody:   "ok",
			nextCalled: true,
			status:     200,
		},
		{
			prefix:     "/api/v0",
			reqURL:     "/api/v1/version",
			nextReqURL: "/version",
			respBody:   "404 page not found\n",
			nextCalled: false,
			status:     404,
		},
	}

	assert := func(name string, exp, real interface{}) {
		if exp != real {
			t.Errorf("expected %s to be %q, but got %q", name, exp, real)
		} else {
			t.Log("ok:", name)
		}
	}

	for _, tc := range tcs {
		var (
			called bool
			h      http.Handler
		)

		r := httptest.NewRequest("", tc.reqURL, nil)
		w := httptest.NewRecorder()
		h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			assert("next r.URL.Path", tc.nextReqURL, r.URL.Path)
			io.WriteString(w, "ok")
		})

		h = newPrefixHandler(tc.prefix, h)
		h.ServeHTTP(w, r)

		assert("called", tc.nextCalled, called)
		assert("response status", tc.status, w.Code)
		assert("response body", tc.respBody, w.Body.String())
	}
}
