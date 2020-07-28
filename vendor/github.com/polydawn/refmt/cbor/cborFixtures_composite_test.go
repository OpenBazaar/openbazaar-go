package cbor

import (
	"testing"

	"github.com/polydawn/refmt/tok/fixtures"
)

func testComposite(t *testing.T) {
	t.Run("array nested in map as non-first and final entry, all indefinite length", func(t *testing.T) {
		seq := fixtures.SequenceMap["array nested in map as non-first and final entry"].SansLengthInfo()
		canon := bcat(b(0xbf),
			b(0x60+2), []byte(`k1`), b(0x60+2), []byte(`v1`),
			b(0x60+2), []byte(`ke`), bcat(b(0x9f),
				b(0x60+2), []byte(`oh`),
				b(0x60+4), []byte(`whee`),
				b(0x60+3), []byte(`wow`),
				b(0xff),
			),
			b(0xff),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("array nested in map as first and non-final entry, all indefinite length", func(t *testing.T) {
		seq := fixtures.SequenceMap["array nested in map as first and non-final entry"].SansLengthInfo()
		canon := bcat(b(0xbf),
			b(0x60+2), []byte(`ke`), bcat(b(0x9f),
				b(0x60+2), []byte(`oh`),
				b(0x60+4), []byte(`whee`),
				b(0x60+3), []byte(`wow`),
				b(0xff),
			),
			b(0x60+2), []byte(`k1`), b(0x60+2), []byte(`v1`),
			b(0xff),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("maps nested in array", func(t *testing.T) {
		seq := fixtures.SequenceMap["maps nested in array"]
		canon := bcat(b(0x80+3),
			bcat(b(0xa0+1),
				b(0x60+1), []byte(`k`), b(0x60+1), []byte(`v`),
			),
			b(0x60+4), []byte(`whee`),
			bcat(b(0xa0+1),
				b(0x60+2), []byte(`k1`), b(0x60+2), []byte(`v1`),
			),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("maps nested in array, all indefinite length", func(t *testing.T) {
		seq := fixtures.SequenceMap["maps nested in array"].SansLengthInfo()
		canon := bcat(b(0x9f),
			bcat(b(0xbf),
				b(0x60+1), []byte(`k`), b(0x60+1), []byte(`v`),
				b(0xff),
			),
			b(0x60+4), []byte(`whee`),
			bcat(b(0xbf),
				b(0x60+2), []byte(`k1`), b(0x60+2), []byte(`v1`),
				b(0xff),
			),
			b(0xff),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("arrays in arrays in arrays", func(t *testing.T) {
		seq := fixtures.SequenceMap["arrays in arrays in arrays"]
		canon := bcat(b(0x80+1), b(0x80+1), b(0x80+0))
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("arrays in arrays in arrays, all indefinite length", func(t *testing.T) {
		seq := fixtures.SequenceMap["arrays in arrays in arrays"].SansLengthInfo()
		canon := bcat(b(0x9f), b(0x9f), b(0x9f), b(0xff), b(0xff), b(0xff))
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("maps nested in maps", func(t *testing.T) {
		seq := fixtures.SequenceMap["maps nested in maps"]
		canon := bcat(b(0xa0+1),
			b(0x60+1), []byte(`k`), bcat(b(0xa0+1),
				b(0x60+2), []byte(`k2`), b(0x60+2), []byte(`v2`),
			),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("maps nested in maps, all indefinite length", func(t *testing.T) {
		seq := fixtures.SequenceMap["maps nested in maps"].SansLengthInfo()
		canon := bcat(b(0xbf),
			b(0x60+1), []byte(`k`), bcat(b(0xbf),
				b(0x60+2), []byte(`k2`), b(0x60+2), []byte(`v2`),
				b(0xff),
			),
			b(0xff),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})

	// Empty and null and null-at-depth...

	t.Run("empty", func(t *testing.T) {
		t.Skip("works, but awkward edge cases in test helpers")
		//seq := fixtures.SequenceMap["empty"]
		//canon := []byte(nil)
		//t.Run("encode canonical", func(t *testing.T) {
		//	checkEncoding(t, seq, canon, nil)
		//})
		//t.Run("decode canonical", func(t *testing.T) {
		//	checkDecoding(t, seq, canon, io.EOF)
		//})
	})
	t.Run("null", func(t *testing.T) {
		seq := fixtures.SequenceMap["null"]
		canon := b(0xf6)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("null in array", func(t *testing.T) {
		seq := fixtures.SequenceMap["null in array"]
		canon := bcat(b(0x80+1),
			b(0xf6),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("null in array, indefinite length", func(t *testing.T) {
		seq := fixtures.SequenceMap["null in array"].SansLengthInfo()
		canon := bcat(b(0x9f),
			b(0xf6),
			b(0xff),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("null in map", func(t *testing.T) {
		seq := fixtures.SequenceMap["null in map"]
		canon := bcat(b(0xa0+1),
			b(0x60+1), []byte(`k`), b(0xf6),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("null in map, indefinite length", func(t *testing.T) {
		seq := fixtures.SequenceMap["null in map"].SansLengthInfo()
		canon := bcat(b(0xbf),
			b(0x60+1), []byte(`k`), b(0xf6),
			b(0xff),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("null in array in array", func(t *testing.T) {
		seq := fixtures.SequenceMap["null in array in array"]
		canon := bcat(b(0x80+1),
			b(0x80+1),
			b(0xf6),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
		t.Run("indefinite length", func(t *testing.T) {
			seq := seq.SansLengthInfo()
			canon := bcat(b(0x9f),
				b(0x9f),
				b(0xf6),
				b(0xff),
				b(0xff),
			)
			t.Run("encode", func(t *testing.T) {
				checkEncoding(t, seq, canon, nil)
			})
			t.Run("decode", func(t *testing.T) {
				checkDecoding(t, seq, canon, nil)
			})
		})
	})
}
