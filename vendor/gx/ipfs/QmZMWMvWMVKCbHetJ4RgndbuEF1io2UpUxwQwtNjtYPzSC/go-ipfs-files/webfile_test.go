package files

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestWebFile(t *testing.T) {
	http.HandleFunc("/my/url/content.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello world!")
	})

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello world!")
	}))
	defer s.Close()

	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	wf := NewWebFile(u)
	body, err := ioutil.ReadAll(wf)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "Hello world!" {
		t.Fatal("should have read the web file")
	}
}
