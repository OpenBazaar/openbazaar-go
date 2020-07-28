package tokconv

type IffyToken interface {
	Type() TokenType
}

type TIMapOpen struct{ Length int }
type TIMapClose struct{}
type TIArrOpen struct{ Length int }
type TIArrClose struct{}
type TINull struct{}
type TIString struct{ Val string }
type TIBytes struct{ Val []byte }
type TIBool struct{ Val bool }
type TIInt struct{ Val int }
type TIUint struct{ Val uint }
type TIFloat64 struct{ Val float64 }

func (TIMapOpen) Type() TokenType  { return TMapOpen }
func (TIMapClose) Type() TokenType { return TMapClose }
func (TIArrOpen) Type() TokenType  { return TArrOpen }
func (TIArrClose) Type() TokenType { return TArrClose }
func (TINull) Type() TokenType     { return TNull }
func (TIString) Type() TokenType   { return TString }
func (TIBytes) Type() TokenType    { return TBytes }
func (TIBool) Type() TokenType     { return TBool }
func (TIInt) Type() TokenType      { return TInt }
func (TIUint) Type() TokenType     { return TUint }
func (TIFloat64) Type() TokenType  { return TFloat64 }

// third option:
//   the content itself is enough in every once of these cases
//    except the opens and closes.
