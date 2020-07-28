package fixtures

import (
	. "github.com/polydawn/refmt/tok"
)

// sequences_Tag contains other primtives wrapped in tags -- but be warned:
// tags are basically a CBOR-specific feature.
//
// The `cbor` package handles tags, as does `obj`, but we recommend avoiding
// use of it because it's the single least widely supported concept that refmt
// acknowledges.
//
// refmt's support of tags is also explicitly limited to single tags per item.
// The CBOR RFC suggests that tags may be nested.  However, this is seen very
// (very) rarely (perhaps never?) in practice, and supporting it would have
// drastic impacts on the memory profile and performance of refmt; therefore,
// we don't and won't.
var sequences_Tag = []Sequence{
	{"tagged object",
		[]Token{
			{Type: TMapOpen, Length: 1, Tagged: true, Tag: 50},
			{Type: TString, Str: "k"},
			{Type: TString, Str: "v"},
			{Type: TMapClose},
		},
	},
	{"tagged string",
		[]Token{
			{Type: TString, Str: "wahoo", Tagged: true, Tag: 50},
		},
	},
	{"array with mixed tagged values",
		[]Token{
			{Type: TArrOpen, Length: 2},
			{Type: TUint, Uint: 400, Tagged: true, Tag: 40},
			{Type: TString, Str: "500", Tagged: true, Tag: 50},
			{Type: TArrClose},
		},
	},
	{"object with deeper tagged values",
		[]Token{
			{Type: TMapOpen, Length: 5},
			{Type: TString, Str: "k1"}, {Type: TString, Str: "500", Tagged: true, Tag: 50},
			{Type: TString, Str: "k2"}, {Type: TString, Str: "untagged"},
			{Type: TString, Str: "k3"}, {Type: TString, Str: "600", Tagged: true, Tag: 60},
			{Type: TString, Str: "k4"}, {Type: TArrOpen, Length: 2},
			/**/ {Type: TString, Str: "asdf", Tagged: true, Tag: 50},
			/**/ {Type: TString, Str: "qwer", Tagged: true, Tag: 50},
			/**/ {Type: TArrClose},
			{Type: TString, Str: "k5"}, {Type: TString, Str: "505", Tagged: true, Tag: 50},
			{Type: TMapClose},
		},
	},
}
