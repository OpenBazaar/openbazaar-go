package cbor

import (
	"testing"

	. "github.com/polydawn/refmt/tok"
	"github.com/polydawn/refmt/tok/fixtures"
)

func testNumber(t *testing.T) {
	t.Run("integer zero", func(t *testing.T) {
		seq := fixtures.Sequence{"integer zero", fixtures.Tokens{{Type: TInt, Int: 0}}}
		canon := deB64("AA==")
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		// No decode test: Impossible to decode to this token because cbor doens't disambiguate positive vs signed ints.
	})
	t.Run("integer zero unsigned", func(t *testing.T) {
		seq := fixtures.Sequence{"integer zero unsigned", fixtures.Tokens{{Type: TUint, Uint: 0}}}
		canon := deB64("AA==")
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("integer one", func(t *testing.T) {
		seq := fixtures.Sequence{"integer one", fixtures.Tokens{{Type: TInt, Int: 1}}}
		canon := deB64("AQ==")
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		// No decode test: Impossible to decode to this token because cbor doens't disambiguate positive vs signed ints.
	})
	t.Run("integer one unsigned", func(t *testing.T) {
		seq := fixtures.Sequence{"integer one unsigned", fixtures.Tokens{{Type: TUint, Uint: 1}}}
		canon := deB64("AQ==")
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("integer neg 1", func(t *testing.T) {
		seq := fixtures.Sequence{"integer neg 1", fixtures.Tokens{{Type: TInt, Int: -1}}}
		canon := deB64("IA==")
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("integer neg 100", func(t *testing.T) {
		seq := fixtures.Sequence{"integer neg 100", fixtures.Tokens{{Type: TInt, Int: -100}}}
		canon := deB64("OGM=")
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	t.Run("integer 1000000", func(t *testing.T) {
		seq := fixtures.Sequence{"integer 1000000", fixtures.Tokens{{Type: TInt, Int: 1000000}}}
		canon := deB64("GgAPQkA=")
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		// No decode test: Impossible to decode to this token because cbor doens't disambiguate positive vs signed ints.
	})
	t.Run("integer 1000000 unsigned", func(t *testing.T) {
		seq := fixtures.Sequence{"integer 1000000 unsigned", fixtures.Tokens{{Type: TUint, Uint: 1000000}}}
		canon := deB64("GgAPQkA=")
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
	//	{"",  // This fixture expects the float32 encoding, and we currently lack support for detecting when things can be safely packed thusly.
	//		fixtures.Sequence{"float decimal e+38", fixtures.Tokens{{Type: TFloat64, Float64: 3.4028234663852886e+38}}},
	//		deB64("+n9///8="),
	//		nil,nil,
	//	},
	t.Run("float 1 e+100", func(t *testing.T) {
		seq := fixtures.Sequence{"float 1 e+100", fixtures.Tokens{{Type: TFloat64, Float64: 1.0e+300}}}
		canon := deB64("+3435DyIAHWc")
		t.Run("encode canonical", func(t *testing.T) {
			checkEncoding(t, seq, canon, nil)
		})
		t.Run("decode canonical", func(t *testing.T) {
			checkDecoding(t, seq, canon, nil)
		})
	})
}
