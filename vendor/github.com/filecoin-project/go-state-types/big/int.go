package big

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"

	cbg "github.com/whyrusleeping/cbor-gen"
)

// BigIntMaxSerializedLen is the max length of a byte slice representing a CBOR serialized big.
const BigIntMaxSerializedLen = 128

type Int struct {
	*big.Int
}

func NewInt(i int64) Int {
	return Int{big.NewInt(0).SetInt64(i)}
}

func NewIntUnsigned(i uint64) Int {
	return Int{big.NewInt(0).SetUint64(i)}
}

func NewFromGo(i *big.Int) Int {
	return Int{big.NewInt(0).Set(i)}
}

func Zero() Int {
	return NewInt(0)
}

// PositiveFromUnsignedBytes interprets b as the bytes of a big-endian unsigned
// integer and returns a positive Int with this absolute value.
func PositiveFromUnsignedBytes(b []byte) Int {
	i := big.NewInt(0).SetBytes(b)
	return Int{i}
}

// MustFromString convers dec string into big integer and panics if conversion
// is not sucessful.
func MustFromString(s string) Int {
	v, err := FromString(s)
	if err != nil {
		panic(err)
	}
	return v
}

func FromString(s string) (Int, error) {
	v, ok := big.NewInt(0).SetString(s, 10)
	if !ok {
		return Int{}, fmt.Errorf("failed to parse string as a big int")
	}

	return Int{v}, nil
}

func (bi Int) Copy() Int {
	return Int{Int: new(big.Int).Set(bi.Int)}
}

func Product(ints ...Int) Int {
	p := NewInt(1)
	for _, i := range ints {
		p = Mul(p, i)
	}
	return p
}

func Mul(a, b Int) Int {
	return Int{big.NewInt(0).Mul(a.Int, b.Int)}
}

func Div(a, b Int) Int {
	return Int{big.NewInt(0).Div(a.Int, b.Int)}
}

func Mod(a, b Int) Int {
	return Int{big.NewInt(0).Mod(a.Int, b.Int)}
}

func Add(a, b Int) Int {
	return Int{big.NewInt(0).Add(a.Int, b.Int)}
}

func Sum(ints ...Int) Int {
	sum := Zero()
	for _, i := range ints {
		sum = Add(sum, i)
	}
	return sum
}

func Subtract(num1 Int, ints ...Int) Int {
	sub := num1
	for _, i := range ints {
		sub = Sub(sub, i)
	}
	return sub
}

func Sub(a, b Int) Int {
	return Int{big.NewInt(0).Sub(a.Int, b.Int)}
}

//  Returns a**e unless e <= 0 (in which case returns 1).
func Exp(a Int, e Int) Int {
	return Int{big.NewInt(0).Exp(a.Int, e.Int, nil)}
}

// Returns x << n
func Lsh(a Int, n uint) Int {
	return Int{big.NewInt(0).Lsh(a.Int, n)}
}

// Returns x >> n
func Rsh(a Int, n uint) Int {
	return Int{big.NewInt(0).Rsh(a.Int, n)}
}

func BitLen(a Int) uint {
	return uint(a.Int.BitLen())
}

func Max(x, y Int) Int {
	// taken from max.Max()
	if x.Equals(Zero()) && x.Equals(y) {
		if x.Sign() != 0 {
			return y
		}
		return x
	}
	if x.GreaterThan(y) {
		return x
	}
	return y
}

func Min(x, y Int) Int {
	// taken from max.Min()
	if x.Equals(Zero()) && x.Equals(y) {
		if x.Sign() != 0 {
			return x
		}
		return y
	}
	if x.LessThan(y) {
		return x
	}
	return y
}

func Cmp(a, b Int) int {
	return a.Int.Cmp(b.Int)
}

// LessThan returns true if bi < o
func (bi Int) LessThan(o Int) bool {
	return Cmp(bi, o) < 0
}

// LessThanEqual returns true if bi <= o
func (bi Int) LessThanEqual(o Int) bool {
	return bi.LessThan(o) || bi.Equals(o)
}

// GreaterThan returns true if bi > o
func (bi Int) GreaterThan(o Int) bool {
	return Cmp(bi, o) > 0
}

// GreaterThanEqual returns true if bi >= o
func (bi Int) GreaterThanEqual(o Int) bool {
	return bi.GreaterThan(o) || bi.Equals(o)
}

// Neg returns the negative of bi.
func (bi Int) Neg() Int {
	return Int{big.NewInt(0).Neg(bi.Int)}
}

// Abs returns the absolute value of bi.
func (bi Int) Abs() Int {
	if bi.GreaterThanEqual(Zero()) {
		return bi.Copy()
	}
	return bi.Neg()
}

// Equals returns true if bi == o
func (bi Int) Equals(o Int) bool {
	return Cmp(bi, o) == 0
}

func (bi *Int) MarshalJSON() ([]byte, error) {
	if bi.Int == nil {
		zero := Zero()
		return json.Marshal(zero)
	}
	return json.Marshal(bi.String())
}

func (bi *Int) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	i, ok := big.NewInt(0).SetString(s, 10)
	if !ok {
		return fmt.Errorf("failed to parse big string: '%s'", string(b))
	}

	bi.Int = i
	return nil
}

func (bi *Int) Bytes() ([]byte, error) {
	if bi.Int == nil {
		return []byte{}, fmt.Errorf("failed to convert to bytes, big is nil")
	}

	switch {
	case bi.Sign() > 0:
		return append([]byte{0}, bi.Int.Bytes()...), nil
	case bi.Sign() < 0:
		return append([]byte{1}, bi.Int.Bytes()...), nil
	default: //  bi.Sign() == 0:
		return []byte{}, nil
	}
}

func FromBytes(buf []byte) (Int, error) {
	if len(buf) == 0 {
		return NewInt(0), nil
	}

	var negative bool
	switch buf[0] {
	case 0:
		negative = false
	case 1:
		negative = true
	default:
		return Zero(), fmt.Errorf("big int prefix should be either 0 or 1, got %d", buf[0])
	}

	i := big.NewInt(0).SetBytes(buf[1:])
	if negative {
		i.Neg(i)
	}

	return Int{i}, nil
}

func (bi *Int) MarshalBinary() ([]byte, error) {
	if bi.Int == nil {
		zero := Zero()
		return zero.Bytes()
	}
	return bi.Bytes()
}

func (bi *Int) UnmarshalBinary(buf []byte) error {
	i, err := FromBytes(buf)
	if err != nil {
		return err
	}

	*bi = i

	return nil
}

func (bi *Int) MarshalCBOR(w io.Writer) error {
	if bi.Int == nil {
		zero := Zero()
		return zero.MarshalCBOR(w)
	}

	enc, err := bi.Bytes()
	if err != nil {
		return err
	}

	encLen := len(enc)
	if encLen > BigIntMaxSerializedLen {
		return fmt.Errorf("big integer byte array too long (%d bytes)", encLen)
	}

	header := cbg.CborEncodeMajorType(cbg.MajByteString, uint64(encLen))
	if _, err := w.Write(header); err != nil {
		return err
	}

	if _, err := w.Write(enc); err != nil {
		return err
	}

	return nil
}

func (bi *Int) UnmarshalCBOR(br io.Reader) error {
	maj, extra, err := cbg.CborReadHeader(br)
	if err != nil {
		return err
	}

	if maj != cbg.MajByteString {
		return fmt.Errorf("cbor input for fil big int was not a byte string (%x)", maj)
	}

	if extra == 0 {
		bi.Int = big.NewInt(0)
		return nil
	}

	if extra > BigIntMaxSerializedLen {
		return fmt.Errorf("big integer byte array too long (%d bytes)", extra)
	}

	buf := make([]byte, extra)
	if _, err := io.ReadFull(br, buf); err != nil {
		return err
	}

	i, err := FromBytes(buf)
	if err != nil {
		return err
	}

	*bi = i

	return nil
}

func (bi *Int) IsZero() bool {
	return bi.Int.Sign() == 0
}

func (bi *Int) Nil() bool {
	return bi.Int == nil
}

func (bi *Int) NilOrZero() bool {
	return bi.Int == nil || bi.Int.Sign() == 0
}
