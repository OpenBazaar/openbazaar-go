package mc_json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	msgio "gx/ipfs/QmcxL9MDzSU5Mj1GcWZD8CXkAFuJXjdbjotZ93o371bKSf/go-msgio"

	mc "gx/ipfs/QmZb2Hc6sQeFpmnSuFLYH2eWjaMcPPtzDzXY1PkMM1sjnP/go-multicodec"
)

var HeaderPath string
var Header []byte
var HeaderMsgioPath string
var HeaderMsgio []byte

func init() {
	HeaderPath = "/json"
	HeaderMsgioPath = "/json/msgio"
	Header = mc.Header([]byte(HeaderPath))
	HeaderMsgio = mc.Header([]byte(HeaderMsgioPath))
}

type codec struct {
	mc    bool
	msgio bool
}

func Codec(msgio bool) mc.Codec {
	return &codec{mc: false, msgio: msgio}
}

func Multicodec(msgio bool) mc.Multicodec {
	return &codec{mc: true, msgio: msgio}
}

func (c *codec) Encoder(w io.Writer) mc.Encoder {
	buf := bytes.NewBuffer(nil)
	return &encoder{
		w:   w,
		c:   c,
		buf: buf,
		enc: json.NewEncoder(buf),
	}
}

func (c *codec) Decoder(r io.Reader) mc.Decoder {
	if !c.mc && !c.msgio {
		// shortcut.
		return json.NewDecoder(r)
	}
	return &decoder{
		r: r,
		c: c,
	}
}

func (c *codec) Header() []byte {
	if c.msgio {
		return HeaderMsgio
	}
	return Header
}

type encoder struct {
	w   io.Writer
	c   *codec
	enc *json.Encoder
	buf *bytes.Buffer
}

type decoder struct {
	remainder bytes.Buffer
	r         io.Reader
	c         *codec
}

func (c *encoder) Encode(v interface{}) error {
	defer c.buf.Reset()
	w := c.w

	if c.c.mc {
		// if multicodec, write the header first
		if _, err := c.w.Write(c.c.Header()); err != nil {
			return err
		}
	}
	if c.c.msgio {
		w = msgio.NewWriter(w)
	}

	// recast to deal with map[interface{}]interface{} case
	vr, err := recast(v)
	if err != nil {
		return err
	}

	if err := c.enc.Encode(vr); err != nil {
		return err
	}

	_, err = io.Copy(w, c.buf)
	return err
}

func (c *decoder) Decode(v interface{}) error {
	// Pick up any leftover bytes from last time.
	r := io.MultiReader(&c.remainder, c.r)

	if c.c.mc {
		// if multicodec, consume the header first
		if err := mc.ConsumeHeader(r, c.c.Header()); err != nil {
			return err
		}
	}
	if c.c.msgio {
		// Might as well read everything up-front to save read calls.
		reader := msgio.NewReader(r)
		msg, err := reader.ReadMsg()
		if err != nil {
			return err
		}
		err = json.Unmarshal(msg, v)
		reader.ReleaseMsg(msg)
		return err
	}

	jDecoder := json.NewDecoder(r)
	err := jDecoder.Decode(v)

	// Put back any additional bytes read.
	var buf bytes.Buffer

	// First the ones in the decoder.
	io.Copy(&buf, jDecoder.Buffered())

	// Then any still in the current remainder.
	io.Copy(&buf, &c.remainder)

	// Save them for next time.
	c.remainder = buf

	var oneByte [1]byte
	io.MultiReader(&c.remainder, c.r).Read(oneByte[:])
	if oneByte[0] != '\n' {
		return fmt.Errorf("expected newline after json")
	}

	return err
}

func recast(v interface{}) (cv interface{}, err error) {
	switch v.(type) {
	case map[interface{}]interface{}:
		vmi := v.(map[interface{}]interface{})
		vms := make(map[string]interface{}, len(vmi))
		for k, v2 := range vmi {
			ks, ok := k.(string)
			if !ok {
				return v, mc.ErrType
			}

			rv2, err := recast(v2)
			if err != nil {
				return v, err
			}

			vms[ks] = rv2
		}
		return vms, nil
	default:
		return v, nil // hope for the best.
	}
}
