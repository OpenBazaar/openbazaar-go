package cbor

import (
	"testing"

	"github.com/polydawn/refmt/tok/fixtures"
)

func testString(t *testing.T) {
	t.Run("flat string", func(t *testing.T) {
		seq := fixtures.SequenceMap["flat string"]
		canon := bcat(b(0x60+5), []byte(`value`))
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
		t.Run("decode indefinite length, single hunk", func(t *testing.T) {
			checkDecoding(t, seq, bcat(b(0x7f), b(0x60+5), []byte(`value`), b(0xff)), nil)
		})
		t.Run("decode indefinite length, multi hunk", func(t *testing.T) {
			checkDecoding(t, seq, bcat(b(0x7f), b(0x60+2), []byte(`va`), b(0x60+3), []byte(`lue`), b(0xff)), nil)
		})
	})
	t.Run("strings needing escape", func(t *testing.T) {
		seq := fixtures.SequenceMap["strings needing escape"]
		canon := bcat(b(0x60+17), []byte("str\nbroken\ttabbed"))
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
}
