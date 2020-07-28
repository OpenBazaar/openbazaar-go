package fixtures

import (
	. "github.com/polydawn/refmt/tok"
)

// sequences_Null includes both null elements
// and elements which are non-null but explicitly empty;
// it's important to test that the distinction is clear.
var sequences_Null = []Sequence{
	{"empty",
		[]Token{},
	},
	{"null",
		[]Token{
			{Type: TNull},
		},
	},
	{"null in array",
		[]Token{
			{Type: TArrOpen, Length: 1},
			{Type: TNull},
			{Type: TArrClose},
		},
	},
	{"null in map",
		[]Token{
			{Type: TMapOpen, Length: 1},
			TokStr("k"),
			{Type: TNull},
			{Type: TMapClose},
		},
	},
	{"null in array in array",
		[]Token{
			{Type: TArrOpen, Length: 1},
			{Type: TArrOpen, Length: 1},
			{Type: TNull},
			{Type: TArrClose},
			{Type: TArrClose},
		},
	},
	{"null in middle of array",
		[]Token{
			{Type: TArrOpen, Length: 5},
			TokStr("one"),
			{Type: TNull},
			TokStr("three"),
			{Type: TNull},
			TokStr("five"),
			{Type: TArrClose},
		},
	},
}
