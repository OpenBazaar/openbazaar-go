package http

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"gx/ipfs/QmTjNRVt2fvaRFu93keEC7z5M1GS1iH6qZ9227htQioTUY/go-ipfs-cmds"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
	"reflect"
)

var (
	MIMEEncodings = map[string]cmds.EncodingType{
		"application/json": cmds.JSON,
		"application/xml":  cmds.XML,
		"text/plain":       cmds.Text,
	}
)

type Response struct {
	length uint64
	err    *cmdkit.Error

	res *http.Response
	req *cmds.Request

	rr  *responseReader
	dec cmds.Decoder

	initErr *cmdkit.Error
}

func (res *Response) Request() *cmds.Request {
	return res.req
}

func (res *Response) Error() *cmdkit.Error {
	e := res.err
	res.err = nil
	return e
}

func (res *Response) Length() uint64 {
	return res.length
}

func (res *Response) RawNext() (interface{}, error) {
	if res.initErr != nil {
		err := res.initErr
		res.initErr = nil

		return err, nil
	}

	// nil decoder means stream not chunks
	// but only do that once
	if res.dec == nil {
		if res.rr == nil {
			return nil, io.EOF
		}
		rr := res.rr
		res.rr = nil
		return rr, nil
	}

	var value interface{}
	if valueType := reflect.TypeOf(res.req.Command.Type); valueType != nil {
		if valueType.Kind() == reflect.Ptr {
			valueType = valueType.Elem()
		}
		value = reflect.New(valueType).Interface()
	}
	m := &cmds.MaybeError{Value: value}
	err := res.dec.Decode(m)

	// last error was sent as value, now we get the same error from the headers. ignore and EOF!
	if err != nil && res.err != nil && err.Error() == res.err.Error() {
		err = io.EOF
	}

	return m.Get(), err
}

func (res *Response) Next() (interface{}, error) {
	v, err := res.RawNext()
	if err != nil {
		return nil, err
	}

	if err, ok := v.(cmdkit.Error); ok {
		v = &err
	}

	switch val := v.(type) {
	case *cmdkit.Error:
		res.err = val
		return nil, cmds.ErrRcvdError
	case cmds.Single:
		return val.Value, nil
	default:
		return v, nil
	}
}

// responseReader reads from the response body, and checks for an error
// in the http trailer upon EOF, this error if present is returned instead
// of the EOF.
type responseReader struct {
	resp *http.Response
}

func (r *responseReader) Read(b []byte) (int, error) {
	if r == nil || r.resp == nil {
		return 0, io.EOF
	}

	n, err := r.resp.Body.Read(b)

	// reading on a closed response body is as good as an io.EOF here
	if err != nil && strings.Contains(err.Error(), "read on closed response body") {
		err = io.EOF
	}
	if err == io.EOF {
		_ = r.resp.Body.Close()
		trailerErr := r.checkError()
		if trailerErr != nil {
			return n, trailerErr
		}
	}
	return n, err
}

func (r *responseReader) checkError() error {
	if e := r.resp.Trailer.Get(StreamErrHeader); e != "" {
		return errors.New(e)
	}
	return nil
}

func (r *responseReader) Close() error {
	return r.resp.Body.Close()
}
