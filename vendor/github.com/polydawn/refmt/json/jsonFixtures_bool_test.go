package json

import (
	"testing"

	"github.com/polydawn/refmt/tok/fixtures"
)

func testBool(t *testing.T) {
	t.Run("bool true", func(t *testing.T) {
		checkCanonical(t, fixtures.SequenceMap["true"], `true`)
	})
	t.Run("bool false", func(t *testing.T) {
		checkCanonical(t, fixtures.SequenceMap["false"], `false`)
	})
}
