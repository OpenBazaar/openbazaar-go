package json

import (
	"testing"

	"github.com/polydawn/refmt/tok/fixtures"
)

func testComposite(t *testing.T) {
	t.Run("array nested in map as non-first and final entry", func(t *testing.T) {
		seq := fixtures.SequenceMap["array nested in map as non-first and final entry"]
		checkCanonical(t, seq, `{"k1":"v1","ke":["oh","whee","wow"]}`)
	})
	t.Run("array nested in map as first and non-final entry", func(t *testing.T) {
		seq := fixtures.SequenceMap["array nested in map as first and non-final entry"]
		checkCanonical(t, seq, `{"ke":["oh","whee","wow"],"k1":"v1"}`)
	})
	t.Run("maps nested in array", func(t *testing.T) {
		seq := fixtures.SequenceMap["maps nested in array"]
		checkCanonical(t, seq, `[{"k":"v"},"whee",{"k1":"v1"}]`)
	})
	t.Run("arrays in arrays in arrays", func(t *testing.T) {
		seq := fixtures.SequenceMap["arrays in arrays in arrays"]
		checkCanonical(t, seq, `[[[]]]`)
	})
	t.Run("maps nested in maps", func(t *testing.T) {
		seq := fixtures.SequenceMap["maps nested in maps"]
		checkCanonical(t, seq, `{"k":{"k2":"v2"}}`)
	})
}
