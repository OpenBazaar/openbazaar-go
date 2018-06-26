package cmds

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sync"

	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

func NewWriterResponseEmitter(w io.WriteCloser, req *Request, enc func(*Request) func(io.Writer) Encoder) *WriterResponseEmitter {
	re := &WriterResponseEmitter{
		w:   w,
		c:   w,
		req: req,
	}

	if enc != nil {
		re.enc = enc(req)(w)
	}

	return re
}

func NewReaderResponse(r io.Reader, encType EncodingType, req *Request) Response {
	emitted := make(chan struct{})

	return &readerResponse{
		req:     req,
		r:       r,
		encType: encType,
		dec:     Decoders[encType](r),
		emitted: emitted,
	}
}

type readerResponse struct {
	r       io.Reader
	encType EncodingType
	dec     Decoder

	req *Request

	length uint64
	err    *cmdkit.Error

	emitted chan struct{}
	once    sync.Once
}

func (r *readerResponse) Request() *Request {
	return r.req
}

func (r *readerResponse) Error() *cmdkit.Error {
	<-r.emitted

	return r.err
}

func (r *readerResponse) Length() uint64 {
	<-r.emitted

	return r.length
}

func (r *readerResponse) RawNext() (interface{}, error) {
	m := &MaybeError{Value: r.req.Command.Type}
	err := r.dec.Decode(m)
	if err != nil {
		return nil, err
	}

	r.once.Do(func() { close(r.emitted) })

	v := m.Get()
	// because working with pointers to arrays is annoying
	if t := reflect.TypeOf(v); t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Slice {
		v = reflect.ValueOf(v).Elem().Interface()
	}
	return v, nil
}

func (r *readerResponse) Next() (interface{}, error) {
	v, err := r.RawNext()
	if err != nil {
		return nil, err
	}

	if err, ok := v.(cmdkit.Error); ok {
		v = &err
	}

	switch val := v.(type) {
	case *cmdkit.Error:
		r.err = val
		return nil, ErrRcvdError
	case Single:
		return val.Value, nil
	default:
		return v, nil
	}
}

type WriterResponseEmitter struct {
	// TODO maybe make those public?
	w   io.Writer
	c   io.Closer
	enc Encoder
	req *Request

	length *uint64
	err    *cmdkit.Error

	emitted bool
}

func (re *WriterResponseEmitter) SetEncoder(mkEnc func(io.Writer) Encoder) {
	re.enc = mkEnc(re.w)
}

func (re *WriterResponseEmitter) SetError(v interface{}, errType cmdkit.ErrorType) {
	err := re.Emit(&cmdkit.Error{Message: fmt.Sprint(v), Code: errType})
	if err != nil {
		panic(err)
	}
}

func (re *WriterResponseEmitter) SetLength(length uint64) {
	if re.emitted {
		return
	}

	*re.length = length
}

func (re *WriterResponseEmitter) Close() error {
	return re.c.Close()
}

func (re *WriterResponseEmitter) Head() Head {
	return Head{
		Len: *re.length,
		Err: re.err,
	}
}

func (re *WriterResponseEmitter) Emit(v interface{}) error {
	if ch, ok := v.(chan interface{}); ok {
		v = (<-chan interface{})(ch)
	}

	if ch, isChan := v.(<-chan interface{}); isChan {
		for v = range ch {
			err := re.Emit(v)
			if err != nil {
				return err
			}
		}
		return nil
	}

	re.emitted = true

	if _, ok := v.(Single); ok {
		defer re.Close()
	}

	return re.enc.Encode(v)
}

type MaybeError struct {
	Value interface{} // needs to be a pointer
	Error cmdkit.Error

	isError bool
}

func (m *MaybeError) Get() interface{} {
	if m.isError {
		return m.Error
	}
	return m.Value
}

func (m *MaybeError) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, &m.Error)
	if err == nil {
		m.isError = true
		return nil
	}

	if m.Value != nil {
		// make sure we are working with a pointer here
		v := reflect.ValueOf(m.Value)
		if v.Kind() != reflect.Ptr {
			m.Value = reflect.New(v.Type()).Interface()
		}

		err = json.Unmarshal(data, m.Value)
	} else {
		// let the json decoder decode into whatever it finds appropriate
		err = json.Unmarshal(data, &m.Value)
	}

	return err
}
