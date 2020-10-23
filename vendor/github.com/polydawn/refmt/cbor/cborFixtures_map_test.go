package cbor

import (
	"testing"

	"github.com/polydawn/refmt/tok/fixtures"
)

func testMap(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		seq := fixtures.SequenceMap["empty map"]
		canon := bcat(b(0xa0))
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})

	t.Run("empty map, indefinite length", func(t *testing.T) {
		seq := fixtures.SequenceMap["empty map"].SansLengthInfo()
		canon := bcat(b(0xbf), b(0xff))
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})

	t.Run("single row map", func(t *testing.T) {
		seq := fixtures.SequenceMap["single row map"]
		canon := bcat(b(0xa0+1),
			b(0x60+3), []byte(`key`), b(0x60+5), []byte(`value`),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})

	t.Run("single row map, indefinite length", func(t *testing.T) {
		seq := fixtures.SequenceMap["single row map"].SansLengthInfo()
		canon := bcat(b(0xbf),
			b(0x60+3), []byte(`key`), b(0x60+5), []byte(`value`),
			b(0xff),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})

	t.Run("duo row map", func(t *testing.T) {
		seq := fixtures.SequenceMap["duo row map"]
		canon := bcat(b(0xa0+2),
			b(0x60+3), []byte(`key`), b(0x60+5), []byte(`value`),
			b(0x60+2), []byte(`k2`), b(0x60+2), []byte(`v2`),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})

	t.Run("duo row map, indefinite length", func(t *testing.T) {
		seq := fixtures.SequenceMap["duo row map"].SansLengthInfo()
		canon := bcat(b(0xbf),
			b(0x60+3), []byte(`key`), b(0x60+5), []byte(`value`),
			b(0x60+2), []byte(`k2`), b(0x60+2), []byte(`v2`),
			b(0xff),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
}
