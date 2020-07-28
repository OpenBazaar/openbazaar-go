package json

import (
	"io"
	"testing"

	"github.com/polydawn/refmt/tok/fixtures"
)

func testArray(t *testing.T) {
	t.Run("empty array", func(t *testing.T) {
		seq := fixtures.SequenceMap["empty array"]
		checkCanonical(t, seq, `[]`)
		t.Run("decode with extra whitespace", func(t *testing.T) {
			checkDecoding(t, seq, `  [ ] `, nil)
		})
	})
	t.Run("single entry array", func(t *testing.T) {
		seq := fixtures.SequenceMap["single entry array"]
		checkCanonical(t, seq, `["value"]`)
		t.Run("decode with extra whitespace", func(t *testing.T) {
			checkDecoding(t, seq, `  [ "value" ] `, nil)
		})
	})
	t.Run("duo entry array", func(t *testing.T) {
		seq := fixtures.SequenceMap["duo entry array"]
		checkCanonical(t, seq, `["value","v2"]`)
	})
	t.Run("reject dangling arr open", func(t *testing.T) {
		seq := fixtures.SequenceMap["dangling arr open"]
		checkDecoding(t, seq, `[`, io.EOF)
	})
}
