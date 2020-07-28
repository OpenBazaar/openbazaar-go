package tok

import (
	"fmt"
	"testing"

	. "github.com/polydawn/refmt/testutil"
)

func TestTokenEqualityDefn(t *testing.T) {
	tt := []struct {
		tok1 Token
		tok2 Token
		eq   bool
	}{
		// The control constants must equal themselves(!):
		{Token{Type: TMapOpen, Length: -1}, Token{Type: TMapOpen, Length: -1}, true},
		{Token{Type: TMapClose, Length: -1}, Token{Type: TMapClose, Length: -1}, true},
		{Token{Type: TArrOpen, Length: -1}, Token{Type: TArrOpen, Length: -1}, true},
		{Token{Type: TArrClose, Length: -1}, Token{Type: TArrClose, Length: -1}, true},
		{Token{Type: TNull}, Token{Type: TNull}, true},

		// The control constants must not equal each other:
		{Token{Type: TMapOpen, Length: -1}, Token{Type: TMapClose}, false},
		{Token{Type: TMapOpen, Length: -1}, Token{Type: TArrOpen, Length: -1}, false},
		{Token{Type: TArrOpen, Length: -1}, Token{Type: TArrClose}, false},
		{Token{Type: TArrOpen, Length: -1}, Token{Type: TMapClose}, false},
		{Token{Type: TNull}, Token{Type: TMapOpen, Length: -1}, false},
		{Token{Type: TNull}, Token{Type: TArrClose}, false},

		// MapOpen and ArrOpen must compare lengths.  Other control constants need not.
		{Token{Type: TMapOpen, Length: -1}, Token{Type: TMapOpen, Length: 1}, false},
		{Token{Type: TMapClose, Length: -1}, Token{Type: TMapClose, Length: 1}, true},
		{Token{Type: TArrOpen, Length: -1}, Token{Type: TArrOpen, Length: 1}, false},
		{Token{Type: TArrClose, Length: -1}, Token{Type: TArrClose, Length: 1}, true},
		{Token{Type: TNull, Length: -1}, Token{Type: TNull, Length: 1}, true},

		// Other values should behave as you expect:
		{Token{Type: TString, Str: "abc"}, Token{Type: TString, Str: "def"}, false},      // other content is not
		{Token{Type: TString, Str: "abc"}, Token{Type: TString, Str: "abc"}, true},       // self is same
		{Token{Type: TInt, Int: 124}, Token{Type: TInt, Int: 124}, true},                 // self is same
		{Token{Type: TInt, Int: 124}, Token{Type: TInt, Int: 789}, false},                // other content is not
		{Token{Type: TInt, Int: 124}, Token{Type: TString, Str: "abc"}, false},           // totally different types are not same
		{Token{Type: TInt, Int: 124}, Token{Type: TString, Str: "abc", Int: 124}, false}, // ... even if residuals of that value are
		{Token{Type: TBool, Bool: true}, Token{Type: TBool, Bool: true}, true},
		{Token{Type: TBool, Bool: true}, Token{Type: TBool, Bool: false}, false},
		{Token{Type: TBytes, Bytes: []byte{1, 2, 3}}, Token{Type: TBytes, Bytes: []byte{1, 2, 3}}, true},
		{Token{Type: TBytes, Bytes: []byte{1, 2, 3}}, Token{Type: TBytes, Bytes: []byte{4, 5, 0xff}}, false},
		{Token{Type: TInt, Int: 124}, Token{Type: TMapOpen}, false},
		{Token{Type: TMapOpen}, Token{Type: TInt, Int: 124}, false},

		// Invalids aren't equal to anything, including themselves:
		{Token{}, Token{}, false},
	}
	for _, tr := range tt {
		Assert(t, fmt.Sprintf("equality check for %s == %s", tr.tok1, tr.tok2),
			tr.eq, IsTokenEqual(tr.tok1, tr.tok2))
	}
}
