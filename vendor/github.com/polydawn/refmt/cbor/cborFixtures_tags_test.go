package cbor

import (
	"testing"

	"github.com/polydawn/refmt/tok/fixtures"
)

func testTags(t *testing.T) {
	t.Run("tagged object", func(t *testing.T) {
		seq := fixtures.SequenceMap["tagged object"]
		canon := bcat(b(0xc0+(0x20-8)), b(50), b(0xa0+1), b(0x60+1), []byte(`k`), b(0x60+1), []byte(`v`))
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("tagged string", func(t *testing.T) {
		seq := fixtures.SequenceMap["tagged string"]
		canon := bcat(b(0xc0+(0x20-8)), b(50), b(0x60+5), []byte(`wahoo`))
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("array with mixed tagged values", func(t *testing.T) {
		seq := fixtures.SequenceMap["array with mixed tagged values"]
		canon := bcat(b(0x80+2),
			b(0xc0+(0x20-8)), b(40), b(0x00+(0x19)), []byte{0x1, 0x90},
			b(0xc0+(0x20-8)), b(50), b(0x60+3), []byte(`500`),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("object with deeper tagged values", func(t *testing.T) {
		seq := fixtures.SequenceMap["object with deeper tagged values"]
		canon := bcat(b(0xa0+5),
			b(0x60+2), []byte(`k1`), b(0xc0+(0x20-8)), b(50), b(0x60+3), []byte(`500`),
			b(0x60+2), []byte(`k2`), b(0x60+8), []byte(`untagged`),
			b(0x60+2), []byte(`k3`), b(0xc0+(0x20-8)), b(60), b(0x60+3), []byte(`600`),
			b(0x60+2), []byte(`k4`), b(0x80+2),
			/**/ b(0xc0+(0x20-8)), b(50), b(0x60+4), []byte(`asdf`),
			/**/ b(0xc0+(0x20-8)), b(50), b(0x60+4), []byte(`qwer`),
			b(0x60+2), []byte(`k5`), b(0xc0+(0x20-8)), b(50), b(0x60+3), []byte(`505`),
		)
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
}
