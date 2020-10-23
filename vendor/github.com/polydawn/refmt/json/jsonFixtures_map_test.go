package json

import (
	"testing"

	"github.com/polydawn/refmt/tok/fixtures"
)

func testMap(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		seq := fixtures.SequenceMap["empty map"]
		checkCanonical(t, seq, `{}`)
		t.Run("decode with extra interior whitespace", func(t *testing.T) {
			checkDecoding(t, seq, `{  }`, nil)
		})
		t.Run("decode with extra flanking whitespace", func(t *testing.T) {
			checkDecoding(t, seq, `  {  }  `, nil)
		})
	})
	t.Run("single row map", func(t *testing.T) {
		seq := fixtures.SequenceMap["single row map"]
		checkCanonical(t, seq, `{"key":"value"}`)
		t.Run("decode with extra whitespace", func(t *testing.T) {
			checkDecoding(t, seq, ` { "key"  :  "value" } `, nil)
		})
	})
	t.Run("duo row map", func(t *testing.T) {
		seq := fixtures.SequenceMap["duo row map"]
		checkCanonical(t, seq, `{"key":"value","k2":"v2"}`)
		t.Run("decode with extra whitespace", func(t *testing.T) {
			checkDecoding(t, seq, `{"key":"value",  "k2":"v2"}`, nil)
		})
		t.Run("decode with trailing comma", func(t *testing.T) {
			checkDecoding(t, seq, `{"key":"value","k2":"v2",}`, nil)
		})
	})
	t.Run("duo row map alt2", func(t *testing.T) {
		seq := fixtures.SequenceMap["duo row map alt2"]
		// note: no encode check here, because this is noncanonical order, so we'd never emit it.
		t.Run("decode noncanonical order", func(t *testing.T) {
			checkDecoding(t, seq, `{"k2":"v2","key":"value"}`, nil)
		})
	})
}
