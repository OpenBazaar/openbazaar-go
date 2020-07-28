package cbor

import (
	"bytes"
	"testing"

	"github.com/polydawn/refmt/tok/fixtures"
)

func testBytes(t *testing.T) {
	t.Run("short byte array", func(t *testing.T) {
		seq := fixtures.SequenceMap["short byte array"]
		canon := bcat(b(0x40+5), []byte(`value`))
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
		t.Run("decode indefinite length, single hunk", func(t *testing.T) {
			checkDecoding(t, seq, bcat(b(0x5f), b(0x40+5), []byte(`value`), b(0xff)), nil)
		})
		t.Run("decode indefinite length, multi hunk", func(t *testing.T) {
			checkDecoding(t, seq, bcat(b(0x5f), b(0x40+2), []byte(`va`), b(0x40+3), []byte(`lue`), b(0xff)), nil)
		})
	})
	t.Run("long zero byte array", func(t *testing.T) {
		seq := fixtures.SequenceMap["long zero byte array"]
		canon := bcat(b(0x40+0x19), []byte{0x1, 0x90}, bytes.Repeat(b(0x0), 400))
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
}
