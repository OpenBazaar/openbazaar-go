package http

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"
	"testing"

	"gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

func TestErrors(t *testing.T) {
	type testcase struct {
		opts       cmdkit.OptMap
		path       []string
		bodyStr    string
		status     string
		errTrailer string
	}

	tcs := []testcase{
		{
			path: []string{"version"},
			bodyStr: `{` +
				`"Version":"0.1.2",` +
				`"Commit":"c0mm17",` +
				`"Repo":"4",` +
				`"System":"` + runtime.GOARCH + "/" + runtime.GOOS + `",` +
				`"Golang":"` + runtime.Version() + `"}` + "\n",
			status: "200 OK",
		},

		// TODO this error should be sent as a value, because it is non-200
		{
			path:    []string{"error"},
			status:  "500 Internal Server Error",
			bodyStr: `{"Message":"an error occurred","Code":0,"Type":"error"}` + "\n",
		},

		{
			path:       []string{"lateerror"},
			status:     "200 OK",
			bodyStr:    `"some value"` + "\n",
			errTrailer: "an error occurred",
		},

		{
			path: []string{"encode"},
			opts: cmdkit.OptMap{
				cmds.EncLong: cmds.Text,
			},
			status:  "500 Internal Server Error",
			bodyStr: "an error occurred",
		},

		{
			path: []string{"lateencode"},
			opts: cmdkit.OptMap{
				cmds.EncLong: cmds.Text,
			},
			status:     "200 OK",
			bodyStr:    "hello\n",
			errTrailer: "an error occurred",
		},

		{
			path: []string{"protoencode"},
			opts: cmdkit.OptMap{
				cmds.EncLong: cmds.Protobuf,
			},
			status:  "500 Internal Server Error",
			bodyStr: `{"Message":"an error occurred","Code":0,"Type":"error"}` + "\n",
		},

		{
			path: []string{"protolateencode"},
			opts: cmdkit.OptMap{
				cmds.EncLong: cmds.Protobuf,
			},
			status:     "200 OK",
			bodyStr:    "hello\n",
			errTrailer: "an error occurred",
		},

		{
			// bad encoding
			path: []string{"error"},
			opts: cmdkit.OptMap{
				cmds.EncLong: "foobar",
			},
			status:  "400 Bad Request",
			bodyStr: `invalid encoding: foobar`,
		},

		{
			path:    []string{"doubleclose"},
			status:  "200 OK",
			bodyStr: `"some value"` + "\n",
		},

		{
			path:    []string{"single"},
			status:  "200 OK",
			bodyStr: `"some value"` + "\n",
		},

		{
			path:    []string{"reader"},
			status:  "200 OK",
			bodyStr: "the reader call returns a reader.",
		},
	}

	mkTest := func(tc testcase) func(*testing.T) {
		return func(t *testing.T) {
			_, srv := getTestServer(t, nil) // handler_test:/^func getTestServer/
			c := NewClient(srv.URL)
			req, err := cmds.NewRequest(context.Background(), tc.path, tc.opts, nil, nil, cmdRoot)
			if err != nil {
				t.Fatal(err)
			}

			httpReq, err := c.(*client).toHTTPRequest(req)
			if err != nil {
				t.Fatal("unexpected error:", err)
			}

			httpClient := http.DefaultClient

			res, err := httpClient.Do(httpReq)
			if err != nil {
				t.Fatal("unexpected error", err)
			}

			if res.Status != tc.status {
				t.Errorf("expected status %v, got %v", tc.status, res.Status)
			}

			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Fatal("err reading response body", err)
			}

			if bodyStr := string(body); bodyStr != tc.bodyStr {
				t.Errorf("expected body string \n\n%v\n\n, got\n\n%v", tc.bodyStr, bodyStr)
			}

			if errTrailer := res.Trailer.Get(StreamErrHeader); errTrailer != tc.errTrailer {
				t.Errorf("expected error header %q, got %q", tc.errTrailer, errTrailer)
			}
		}
	}

	for i, tc := range tcs {
		t.Run(fmt.Sprintf("%d-%s", i, strings.Join(tc.path, "/")), mkTest(tc))
	}
}
