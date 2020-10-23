package cbor

import (
	"testing"

	"github.com/polydawn/refmt/tok/fixtures"
)

func testArray(t *testing.T) {
	t.Run("empty array", func(t *testing.T) {
		seq := fixtures.SequenceMap["empty array"]
		canon := bcat(b(0x80))
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("empty array, indefinite length", func(t *testing.T) {
		seq := fixtures.SequenceMap["empty array"].SansLengthInfo()
		canon := bcat(b(0x9f), b(0xff))
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})

	t.Run("single entry array", func(t *testing.T) {
		seq := fixtures.SequenceMap["single entry array"]
		canon := bcat(b(0x80+1), b(0x60+5), []byte(`value`))
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("single entry array, indefinite length", func(t *testing.T) {
		seq := fixtures.SequenceMap["single entry array"].SansLengthInfo()
		canon := bcat(b(0x9f), b(0x60+5), []byte(`value`), b(0xff))
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
		t.Run("decode with nested indef string", func(t *testing.T) {
			checkDecoding(t, seq,
				bcat(b(0x9f),
					bcat(b(0x7f), b(0x60+5), []byte(`value`), b(0xff)),
					b(0xff)),
				nil)
		})
	})

	t.Run("duo entry array", func(t *testing.T) {
		seq := fixtures.SequenceMap["duo entry array"]
		canon := bcat(b(0x80+2),
			b(0x60+5), []byte(`value`),
			b(0x60+2), []byte(`v2`),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("duo entry array, indefinite length", func(t *testing.T) {
		seq := fixtures.SequenceMap["duo entry array"].SansLengthInfo()
		canon := bcat(b(0x9f),
			b(0x60+5), []byte(`value`),
			b(0x60+2), []byte(`v2`),
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
