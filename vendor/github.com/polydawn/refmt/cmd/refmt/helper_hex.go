package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
)

const hextable = "0123456789abcdef"

/*
	Convert hex into bytes, returning another reader.

	It's *not* streaming, though.  (PRs welcome...)
*/
func hexReader(hr io.Reader) io.Reader {
	in, err := ioutil.ReadAll(hr)
	if err != nil {
		return errthunkReader{err}
	}
	bs := make([]byte, len(in)/2)
	n, err := hex.Decode(bs, in)
	if err != nil {
		return errthunkReader{err}
	}
	if n != len(in)/2 {
		return errthunkReader{fmt.Errorf("hex len mismatch: %d chars became %d bytes", len(in), n)}
	}
	return bytes.NewBuffer(bs)
}

type errthunkReader struct {
	err error
}

func (r errthunkReader) Read([]byte) (int, error) {
	return 0, r.err
}

type hexWriter struct {
	w io.Writer
}

func (hw hexWriter) Write(src []byte) (int, error) {
	dst := make([]byte, len(src)*2)
	for i, v := range src {
		dst[i*2] = hextable[v>>4]
		dst[i*2+1] = hextable[v&0x0f]
	}
	n, err := hw.w.Write(dst)
	return n / 2, err
}
