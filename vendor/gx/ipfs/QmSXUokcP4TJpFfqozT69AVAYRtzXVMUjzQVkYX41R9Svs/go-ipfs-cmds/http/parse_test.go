package http

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	cmds "gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

func TestParse(t *testing.T) {
	root := &cmds.Command{
		Subcommands: map[string]*cmds.Command{
			"block": &cmds.Command{
				Subcommands: map[string]*cmds.Command{
					"put": &cmds.Command{
						Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
							defer resp.Close()
							resp.Emit("done")
							return nil
						},
					},
				},
			},
		},
	}

	r, err := http.NewRequest("GET", "/block/put", nil)
	if err != nil {
		t.Fatal(err)
	}
	req, err := parseRequest(nil, r, root)
	if err != nil {
		t.Fatal(err)
	}

	pth := req.Path
	if pth[0] != "block" || pth[1] != "put" || len(pth) != 2 {
		t.Errorf("incorrect path %v, expected %v", pth, []string{"block", "put"})
	}

	r, err = http.NewRequest("GET", "/block/bla", nil)
	if err != nil {
		t.Fatal(err)
	}
	req, err = parseRequest(nil, r, root)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

type parseReqTestCase struct {
	path string
	opts url.Values
	body io.Reader

	cmdsReq *cmds.Request
	err     error
}

func (tc parseReqTestCase) test(t *testing.T) {
	var vs = url.Values{}

	for k, opts := range tc.opts {
		for _, opt := range opts {
			vs.Add(k, opt)
		}
	}

	// we're just parsing the request, so the host part of the url doens't really matter
	httpReq, err := http.NewRequest("GET", "http://127.0.0.1:5001"+tc.path, tc.body)
	if err != nil {
		t.Fatal(err)
	}
	httpReq.URL.RawQuery = vs.Encode()

	req, err := parseRequest(nil, httpReq, cmdRoot)
	if !errEq(err, tc.err) {
		t.Fatalf("expected error to be %v, but got %v", tc.err, err)
	}
	if err != nil {
		return
	}

	if req.Command != tc.cmdsReq.Command {
		t.Errorf("expected req.Command to be\n%v\n but got\n%v", tc.cmdsReq.Command, req.Command)
	}

	if !reflect.DeepEqual(req.Path, tc.cmdsReq.Path) {
		t.Errorf("expected req.Path to be %v, but got %v", tc.cmdsReq.Path, req.Path)
	}

	if !reflect.DeepEqual(req.Arguments, tc.cmdsReq.Arguments) {
		t.Errorf("expected req.Arguments to be %v, but got %v", tc.cmdsReq.Arguments, req.Arguments)
	}

	if !reflect.DeepEqual(req.Options, tc.cmdsReq.Options) {
		t.Errorf("expected req.Options to be %v, but got %v", tc.cmdsReq.Options, req.Options)
	}
}

func TestParseRequest(t *testing.T) {
	tcs := []parseReqTestCase{
		{
			path: "/version",
			opts: url.Values{
				"all": []string{"true"},
			},
			cmdsReq: &cmds.Request{
				Command:   cmdRoot.Subcommands["version"],
				Path:      []string{"version"},
				Arguments: []string{},
				Options: cmdkit.OptMap{
					"all":        true,
					cmds.EncLong: cmds.JSON,
				},
			},
		},
	}

	for _, tc := range tcs {
		tc.test(t)
	}
}

type parseRespTestCase struct {
	status int
	header http.Header
	body   io.ReadCloser

	values []interface{}
	err    error
}

func (tc parseRespTestCase) test(t *testing.T) {
	httpResp := &http.Response{
		StatusCode: tc.status,
		Header:     tc.header,
		Body:       tc.body,
	}

	resp, err := parseResponse(httpResp, &cmds.Request{Command: cmdRoot.Subcommands["version"]})
	if !errEq(err, tc.err) {
		t.Fatalf("expected error to be %v, but got %v", tc.err, err)
	}
	if err != nil {
		return
	}

	t.Log(resp.(*Response).dec)

	for _, v := range tc.values {
		val, err := resp.Next()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(v, val) {
			t.Fatalf("expected %v(%T) but got %v(%T)", v, v, val, val)
		}
	}

	_, err = resp.Next()
	if err != io.EOF {
		t.Fatalf("expected EOF but got %v", err)
	}
}

type fakeCloser struct {
	io.Reader
}

func (c fakeCloser) Close() error { return nil }

func mkbuf(str string) io.ReadCloser {
	buf := bytes.NewBuffer(nil)
	buf.WriteString(str)
	return fakeCloser{buf}
}

func TestParseResponse(t *testing.T) {
	tcs := []parseRespTestCase{
		{
			status: 200,
			header: http.Header{
				contentTypeHeader: []string{"application/json"},
				channelHeader:     []string{"1"},
			},
			body: mkbuf(`{"Version":"0.1.2", "Commit":"c0mm17", "Repo":"4"}`),
			values: []interface{}{
				&VersionOutput{
					Version: "0.1.2",
					Commit:  "c0mm17",
					Repo:    "4",
				},
			},
		},
		{
			status: 500,
			header: http.Header{
				contentTypeHeader: []string{"evil/bad"},
				channelHeader:     []string{"1"},
			},
			body: mkbuf("test error"),
			err:  fmt.Errorf("unknown error content type: %s", "evil/bad"),
		},
	}

	for _, tc := range tcs {
		tc.test(t)
	}
}
