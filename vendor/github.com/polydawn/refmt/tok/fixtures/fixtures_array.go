package fixtures

import (
	. "github.com/polydawn/refmt/tok"
)

var sequences_Array = []Sequence{
	{"empty array",
		[]Token{
			{Type: TArrOpen, Length: 0},
			{Type: TArrClose},
		},
	},
	{"single entry array",
		[]Token{
			{Type: TArrOpen, Length: 1},
			TokStr("value"),
			{Type: TArrClose},
		},
	},
	{"duo entry array",
		[]Token{
			{Type: TArrOpen, Length: 2},
			TokStr("value"),
			TokStr("v2"),
			{Type: TArrClose},
		},
	},

	// Partial sequences!
	// Decoders may emit these before hitting an error (like EOF, or invalid following serial token).
	// Encoders may consume these, but ending after them would be an unexpected end of sequence error.

	{"dangling arr open",
		[]Token{
			{Type: TArrOpen, Length: 1},
			{}, // The error step yields an invalid token.
		},
	},
}
