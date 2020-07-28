package fixtures

import (
	"github.com/polydawn/refmt/tok"
)

// TokenSource and TokenSink implementations often leave a mess behind.
// They're built around mutating a *Token they're given, because careful
// memory management is critically important to overall performance in a
// serialization/deserialization system.
// They're also written to set precisely the fields of interest on a *Token,
// and no more -- despite the fact some fields are conditionally relevant,
// and also that we used an expanded union pattern to talk to the Go compiler
// about how we want to use memory without getting in an argument about
// primtive types and boxing (here's looking at you, `runtime.convT2E`).
// Thus: reusing a Token over time will cause it to accumulate state in
// memory which is not currently interesting!
//
// Ex: json decoding a map `{}` will yield two tokens with Length properties
// of `-1` -- not because that's *correct* nor meaningful for the second
// token, but because *it doesn't matter* and therefore we don't waste time
// re-zeroing the Length field.
//
// So, it comes up that we'd like to re-normalize some of these things
// when operating on tokens in test fixtures.  The shippable logic
// already understands all the relevant semiotics here (or it wouldn't
// pass tests anyway), but our comparators don't want to replicate it;
// easier to implement normalization than the equivalent logic in
// branching comparison methods (which then need more custom printing, etc).

// StompLengths in-place mutates all tokens to have length=-1 if length is
// a relevant property for that Token.Type, and to 0 otherwise.
func StompLengths(v []tok.Token) {
	for i := range v {
		switch v[i].Type {
		case tok.TMapOpen, tok.TArrOpen:
			v[i].Length = -1
		case tok.TString, tok.TBytes:
			// Note that we don't regard length as relevant to TString nor TBytes,
			// because those tokens already carry that info in their golang types
			// along with the actual data.
			fallthrough
		default:
			v[i].Length = 0
		}
	}
}
