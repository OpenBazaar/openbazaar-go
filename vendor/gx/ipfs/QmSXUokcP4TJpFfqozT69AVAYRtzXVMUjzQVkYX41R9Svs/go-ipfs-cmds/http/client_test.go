package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cmds "gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds"
)

func TestClientUserAgent(t *testing.T) {
	type testcase struct {
		host string
		ua   string
		path []string
	}

	tcs := []testcase{
		{ua: "/go-ipfs/0.4", path: []string{"version"}},
	}

	for _, tc := range tcs {
		var called bool

		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			t.Log(r)

			if ua := r.Header.Get("User-Agent"); ua != tc.ua {
				t.Errorf("expected user agent %q, got %q", tc.ua, ua)
			}

			expPath := "/" + strings.Join(tc.path, "/")
			if path := r.URL.Path; path != expPath {
				t.Errorf("expected path %q, got %q", expPath, path)
			}

			w.WriteHeader(http.StatusOK)
		}))
		testClient := s.Client()
		tc.host = s.URL
		r := &cmds.Request{Path: tc.path, Command: &cmds.Command{}, Root: &cmds.Command{}}

		c := NewClient(tc.host, ClientWithUserAgent(tc.ua)).(*client)
		c.httpClient = testClient
		c.Send(r)

		if !called {
			t.Error("handler has not been called")
		}
	}
}

func TestClientAPIPrefix(t *testing.T) {
	type testcase struct {
		host   string
		prefix string
		path   []string
	}

	tcs := []testcase{
		{prefix: "/api/v0", path: []string{"version"}},
		{prefix: "/api/v1", path: []string{"version"}},
	}

	for _, tc := range tcs {
		var called bool

		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			t.Log(r)

			expPath := tc.prefix + "/" + strings.Join(tc.path, "/")
			if path := r.URL.Path; path != expPath {
				t.Errorf("expected path %q, got %q", expPath, path)
			}

			w.WriteHeader(http.StatusOK)
		}))
		testClient := s.Client()
		tc.host = s.URL
		r := &cmds.Request{Path: tc.path, Command: &cmds.Command{}, Root: &cmds.Command{}}

		c := NewClient(tc.host, ClientWithAPIPrefix(tc.prefix)).(*client)
		c.httpClient = testClient
		c.Send(r)

		if !called {
			t.Error("handler has not been called")
		}
	}
}
