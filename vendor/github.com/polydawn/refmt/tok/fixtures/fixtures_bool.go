package fixtures

import (
	. "github.com/polydawn/refmt/tok"
)

var sequences_Bool = []Sequence{
	{"true",
		[]Token{
			{Type: TBool, Bool: true},
		},
	},
	{"false",
		[]Token{
			{Type: TBool, Bool: false},
		},
	},
}
