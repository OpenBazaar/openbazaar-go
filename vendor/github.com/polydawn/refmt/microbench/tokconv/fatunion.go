package tokconv

type FatToken struct {
	// The type of token.  Indicates which of the value fields has meaning,
	// or has a special value to indicate beginnings and endings of maps and arrays.
	Type   TokenType
	Length int // If this is a TMapOpen or TArrOpen, a length may be specified.  Use -1 for unknown.

	Str     string  // Value union.  Only one of these has meaning, depending on the value of 'Type'.
	Bytes   []byte  // Value union.  Only one of these has meaning, depending on the value of 'Type'.
	Bool    bool    // Value union.  Only one of these has meaning, depending on the value of 'Type'.
	Int     int64   // Value union.  Only one of these has meaning, depending on the value of 'Type'.
	Uint    uint64  // Value union.  Only one of these has meaning, depending on the value of 'Type'.
	Float64 float64 // Value union.  Only one of these has meaning, depending on the value of 'Type'.

	Tagged bool // Extension slot for cbor.
	Tag    int  // Extension slot for cbor.  Only applicable if tagged=true.
}

type TokenType byte

const (
	TMapOpen  TokenType = '{'
	TMapClose TokenType = '}'
	TArrOpen  TokenType = '['
	TArrClose TokenType = ']'
	TNull     TokenType = '0'

	TString  TokenType = 's'
	TBytes   TokenType = 'x'
	TBool    TokenType = 'b'
	TInt     TokenType = 'i'
	TUint    TokenType = 'u'
	TFloat64 TokenType = 'f'
)
