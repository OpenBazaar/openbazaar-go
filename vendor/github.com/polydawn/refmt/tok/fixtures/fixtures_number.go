package fixtures

import (
	. "github.com/polydawn/refmt/tok"
)

// sequences_Number contains what it says on the tin -- but be warned:
// numbers are a surprisingly contentious topic.
//
// CBOR can't distinguish between positive numbers and unsigned;
// JSON can't generally distinguish much of anything from anything, and is
// subject to disasterous issues around floating point precision.
//
// Most of the numeric token sequences are thus commented out because
// unfortunately they're functionally useless; the serialization packages
// define their own numeric fixtures locally to target their own vagueries.
// The few fixtures here are mostly those used in the obj package tests.
var sequences_Number = []Sequence{
	//	{"integer zero", []Token{{Type: TInt, Int: 0}}},
	{"integer one", []Token{{Type: TInt, Int: 1}}},
	//	{"integer neg one", []Token{{Type: TInt, Int: -1}}},
}
