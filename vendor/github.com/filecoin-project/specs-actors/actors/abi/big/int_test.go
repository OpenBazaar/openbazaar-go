package big

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBigIntSerializationRoundTrip(t *testing.T) {
	testValues := []string{
		"0", "1", "10", "-10", "9999", "12345678901234567891234567890123456789012345678901234567890",
	}

	for _, v := range testValues {
		bi, err := FromString(v)
		if err != nil {
			t.Fatal(err)
		}

		buf := new(bytes.Buffer)
		if err := bi.MarshalCBOR(buf); err != nil {
			t.Fatal(err)
		}

		var out Int
		if err := out.UnmarshalCBOR(buf); err != nil {
			t.Fatal(err)
		}

		if Cmp(out, bi) != 0 {
			t.Fatal("failed to round trip Int through cbor")
		}

	}

	// nil check
	bi := Int{}
	var buf bytes.Buffer
	err := bi.MarshalCBOR(&buf)
	require.NoError(t, err)

	assert.Equal(t, "@", buf.String())

}

func TestNewInt(t *testing.T) {
	a := int64(999)
	ta := NewInt(a)
	b := big.NewInt(999)
	tb := Int{Int: b}
	assert.True(t, ta.Equals(tb))
	assert.Equal(t, "999", ta.String())
}

func TestInt_MarshalUnmarshalJSON(t *testing.T) {
	ta := NewInt(54321)
	tb := NewInt(0)

	res, err := ta.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, "\"54321\"", string(res[:]))

	require.NoError(t, tb.UnmarshalJSON(res))
	assert.Equal(t, ta, tb)

	assert.EqualError(t, tb.UnmarshalJSON([]byte("123garbage"[:])), "invalid character 'g' after top-level value")

	tnil := Int{}
	s, err := tnil.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, "\"0\"", string(s))
}

func TestOperations(t *testing.T) {
	testCases := []struct {
		name     string
		f        func(Int, Int) Int
		expected Int
	}{
		{name: "Sum", f: Add, expected: NewInt(7000)},
		{name: "Sub", f: Sub, expected: NewInt(3000)},
		{name: "Mul", f: Mul, expected: NewInt(10000000)},
		{name: "Div", f: Div, expected: NewInt(2)},
		{name: "Mod", f: Mod, expected: NewInt(1000)},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ta := Int{Int: big.NewInt(5000)}
			tb := Int{Int: big.NewInt(2000)}
			assert.Equal(t, testCase.expected, testCase.f(ta, tb))
		})
	}

	ta := NewInt(5000)
	tb := NewInt(2000)
	tc := NewInt(2000)
	assert.Equal(t, Cmp(ta, tb), 1)
	assert.Equal(t, Cmp(tb, ta), -1)
	assert.Equal(t, Cmp(tb, tc), 0)
	assert.True(t, ta.GreaterThan(tb))
	assert.False(t, ta.LessThan(tb))
	assert.True(t, tb.Equals(tc))

	ta = Int{}
	assert.True(t, ta.Nil())
}

func TestSum(t *testing.T) {
	b1 := NewInt(1)
	b2 := NewInt(2)
	b3 := NewInt(3)
	b4 := NewInt(4)

	require.EqualValues(t, NewInt(10), Sum(b1, b2, b3, b4))

	require.EqualValues(t, NewInt(20), Sum(NewInt(20)))
}

func TestInt_Format(t *testing.T) {
	ta := NewInt(33333000000)

	s := fmt.Sprintf("%s", ta) // nolint: gosimple
	assert.Equal(t, "33333000000", s)

	s1 := fmt.Sprintf("%v", ta) // nolint: gosimple
	assert.Equal(t, "33333000000", s1)

	s2 := fmt.Sprintf("%-15d", ta) // nolint: gosimple
	assert.Equal(t, "33333000000    ", s2)
}

func TestPositveFromUnsignedBytes(t *testing.T) {
	res := PositiveFromUnsignedBytes([]byte("garbage"[:]))
	// garbage in, garbage out
	expected := Int{Int: big.NewInt(29099066505914213)}
	assert.Equal(t, expected, res)

	expected2 := Int{Int: big.NewInt(12345)}
	expectedRes := expected2.Int.Bytes()
	res = PositiveFromUnsignedBytes(expectedRes)
	assert.Equal(t, expected2, res)
	assert.Equal(t, 1, res.Sign()) // positive
}

func TestFromString(t *testing.T) {
	_, err := FromString("garbage")
	assert.EqualError(t, err, "failed to parse string as a big int")

	res, err := FromString("12345")
	require.NoError(t, err)
	expected := Int{Int: big.NewInt(12345)}
	assert.Equal(t, expected, res)
}
