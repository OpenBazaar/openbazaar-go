package http

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	cmds "gx/ipfs/QmTjNRVt2fvaRFu93keEC7z5M1GS1iH6qZ9227htQioTUY/go-ipfs-cmds"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

var (
	HeadRequest = fmt.Errorf("HEAD request")

	AllowedExposedHeadersArr = []string{streamHeader, channelHeader, extraContentLengthHeader}
	AllowedExposedHeaders    = strings.Join(AllowedExposedHeadersArr, ", ")

	mimeTypes = map[cmds.EncodingType]string{
		cmds.Protobuf: "application/protobuf",
		cmds.JSON:     "application/json",
		cmds.XML:      "application/xml",
		cmds.Text:     "text/plain",
	}
)

// NewResponeEmitter returns a new ResponseEmitter.
func NewResponseEmitter(w http.ResponseWriter, method string, req *cmds.Request) ResponseEmitter {
	encType := cmds.GetEncoding(req)

	var enc cmds.Encoder

	if _, ok := cmds.Encoders[encType]; ok {
		enc = cmds.Encoders[encType](req)(w)
	}

	re := &responseEmitter{
		w:       w,
		encType: encType,
		enc:     enc,
		method:  method,
		req:     req,
	}
	return re
}

type ResponseEmitter interface {
	cmds.ResponseEmitter
	http.Flusher
}

type responseEmitter struct {
	w http.ResponseWriter

	enc     cmds.Encoder
	encType cmds.EncodingType
	req     *cmds.Request

	length uint64
	err    *cmdkit.Error

	streaming bool
	once      sync.Once
	method    string
}

func (re *responseEmitter) Emit(value interface{}) error {
	ch, isChan := value.(<-chan interface{})
	if !isChan {
		ch, isChan = value.(chan interface{})
	}

	if isChan {
		for value = range ch {
			err := re.Emit(value)
			if err != nil {
				return err
			}
		}
		return nil
	}

	var err error

	re.once.Do(func() { re.preamble(value) })

	// return immediately if this is a head request
	if re.method == "HEAD" {
		return nil
	}

	if single, ok := value.(cmds.Single); ok {
		value = single.Value
		defer re.Close()
	}

	if re.w == nil {
		return fmt.Errorf("connection already closed / custom - http.respem - TODO")
	}

	// ignore those
	if value == nil {
		return nil
	}

	if _, ok := value.(cmdkit.Error); ok {
		log.Warning("fixme: got Error not *Error: ", value)
		value = &value
	}

	switch v := value.(type) {
	case io.Reader:
		err = flushCopy(re.w, v)
	case *cmdkit.Error:
		if re.streaming || v.Code == cmdkit.ErrFatal {
			// abort by sending an error trailer
			re.w.Header().Add(StreamErrHeader, v.Error())
		} else {
			err = re.enc.Encode(value)
		}
	default:
		err = re.enc.Encode(value)
	}

	if f, ok := re.w.(http.Flusher); ok {
		f.Flush()
	}

	if err != nil {
		log.Error(err)
	}

	return err
}

func (re *responseEmitter) SetLength(l uint64) {
	h := re.w.Header()
	h.Set("X-Content-Length", strconv.FormatUint(l, 10))

	re.length = l
}

func (re *responseEmitter) Close() error {
	re.once.Do(func() { re.preamble(nil) })
	return nil
}

func (re *responseEmitter) SetError(v interface{}, errType cmdkit.ErrorType) {
	err := re.Emit(&cmdkit.Error{Message: fmt.Sprint(v), Code: errType})
	if err != nil {
		log.Debug("http.SetError err=", err)
		panic(err)
	}
}

// Flush the http connection
func (re *responseEmitter) Flush() {
	re.once.Do(func() { re.preamble(nil) })

	if flusher, ok := re.w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (re *responseEmitter) preamble(value interface{}) {
	status := http.StatusOK
	h := re.w.Header()

	// unpack value if it needs special treatment in the type switch below
	if s, isSingle := value.(cmds.Single); isSingle {
		if err, isErr := s.Value.(cmdkit.Error); isErr {
			value = &err
		}

		if err, isErr := s.Value.(*cmdkit.Error); isErr {
			value = err
		}
	}

	var mime string

	switch v := value.(type) {
	case *cmdkit.Error:
		err := v
		if err.Code == cmdkit.ErrClient {
			status = http.StatusBadRequest
		} else {
			status = http.StatusInternalServerError
		}

		// if this is not a head request, the error will be sent as a trailer or as a value
		if re.method == "HEAD" {
			http.Error(re.w, err.Error(), status)
			re.w = nil

			return
		}
	case io.Reader:
		// set streams output type to text to avoid issues with browsers rendering
		// html pages on priveleged api ports
		h.Set(streamHeader, "1")
		re.streaming = true

		mime = "text/plain"
	case cmds.Single:
	case nil:
		h.Set(channelHeader, "1")
	default:
		h.Set(channelHeader, "1")
	}

	// Set up our potential trailer
	h.Set("Trailer", StreamErrHeader)

	if mime == "" {
		var ok bool

		// lookup mime type from map
		mime, ok = mimeTypes[re.encType]
		if !ok {
			// catch-all, set to text as default
			mime = "text/plain"
		}
	}

	h.Set(contentTypeHeader, mime)

	// set 'allowed' headers
	h.Set("Access-Control-Allow-Headers", AllowedExposedHeaders)
	// expose those headers
	h.Set("Access-Control-Expose-Headers", AllowedExposedHeaders)

	re.w.WriteHeader(status)
}

type responseWriterer interface {
	Lower() http.ResponseWriter
}

func (re *responseEmitter) SetEncoder(enc func(io.Writer) cmds.Encoder) {
	re.enc = enc(re.w)
}

func flushCopy(w io.Writer, r io.Reader) error {
	buf := make([]byte, 4096)
	f, ok := w.(http.Flusher)
	if !ok {
		_, err := io.Copy(w, r)
		return err
	}
	for {
		n, err := r.Read(buf)
		switch err {
		case io.EOF:
			if n <= 0 {
				return nil
			}
			// if data was returned alongside the EOF, pretend we didnt
			// get an EOF. The next read call should also EOF.
		case nil:
			// continue
		default:
			return err
		}

		nw, err := w.Write(buf[:n])
		if err != nil {
			return err
		}

		if nw != n {
			return fmt.Errorf("http write failed to write full amount: %d != %d", nw, n)
		}

		f.Flush()
	}
}
