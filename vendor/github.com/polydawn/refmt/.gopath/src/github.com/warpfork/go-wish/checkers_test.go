package wish

import (
	"fmt"
	"testing"
)

// helper function for testing our testing tools since we can't use our testing tools!
func shouldStringMatch(t *testing.T, actual, desire string) {
	if actual != desire {
		t.Errorf("actual:\n%s\ndesire:\n%s\n", Indent(actual), Indent(desire))
	}
}

func TestShouldEqual(t *testing.T) {
	shouldEqual := func(a, d interface{}) string {
		msg, _ := ShouldEqual(a, d)
		return msg
	}
	t.Run("equivalent ints", func(t *testing.T) {
		shouldStringMatch(t, shouldEqual(
			int(1),
			int(1),
		), "")
	})
	t.Run("distinct ints", func(t *testing.T) {
		shouldStringMatch(t, shouldEqual(
			int(1),
			int(2),
		), Dedent(`
			  int(
			- 	1,
			+ 	2,
			  )
		`))
	})
	t.Run("distinct numeric types", func(t *testing.T) {
		shouldStringMatch(t, shouldEqual(
			int(1),
			uint(1),
		), Dedent(`
			  interface{}(
			- 	int(1),
			+ 	uint(0x01),
			  )
		`))
	})
	t.Run("equivalent strings", func(t *testing.T) {
		shouldStringMatch(t, shouldEqual(
			"asdf",
			"asdf",
		), "")
	})
	t.Run("distinct short strings", func(t *testing.T) {
		shouldStringMatch(t, shouldEqual(
			"asdf",
			"asdx",
		), Dedent(`
			@@ -1 +1 @@
			- asdf
			+ asdx
		`))
	})
	t.Run("fmt.Errorf errors", func(t *testing.T) {
		t.Run("compared to equal others of their type", func(t *testing.T) {
			shouldStringMatch(t, shouldEqual(
				fmt.Errorf("error zed"),
				fmt.Errorf("error zed"),
			), "")
		})
		t.Run("compared to distinct others of their type", func(t *testing.T) {
			shouldStringMatch(t, shouldEqual(
				fmt.Errorf("error zed"),
				fmt.Errorf("error fwing"),
			), Dedent(`
				  &errors.errorString{
				- 	s: "error zed",
				+ 	s: "error fwing",
				  }
			`))
		})
		t.Run("compared to nil", func(t *testing.T) { // This one's weird.  Doesn't explicitly say 'nil' and also is treating errors magically >:/
			shouldStringMatch(t, shouldEqual(
				fmt.Errorf("error zed"),
				nil,
			), Dedent(`
				  interface{}(
				- 	e"error zed",
				  )
			`))
		})
	})
}
