package refmt_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/polydawn/refmt"
	"github.com/polydawn/refmt/cbor"
	"github.com/polydawn/refmt/json"
	"github.com/polydawn/refmt/obj/atlas"
)

func TestRoundTrip(t *testing.T) {
	t.Run("nil nil", func(t *testing.T) {
		testRoundTripAllEncodings(t, nil, atlas.MustBuild())
	})
	t.Run("empty []interface{}", func(t *testing.T) {
		testRoundTripAllEncodings(t, []interface{}{}, atlas.MustBuild())
	})
	t.Run("nil []byte{}", func(t *testing.T) {
		testRoundTripAllEncodings(t, []byte(nil), atlas.MustBuild())
	})
	t.Run("nil []interface{}", func(t *testing.T) {
		testRoundTripAllEncodings(t, []interface{}(nil), atlas.MustBuild())
	})
	t.Run("empty map[string]interface{}", func(t *testing.T) {
		testRoundTripAllEncodings(t, map[string]interface{}(nil), atlas.MustBuild())
	})
	t.Run("nil map[string]interface{}", func(t *testing.T) {
		testRoundTripAllEncodings(t, map[string]interface{}(nil), atlas.MustBuild())
	})
	t.Run("4-value []interface{str}", func(t *testing.T) {
		testRoundTripAllEncodings(t, []interface{}{"str", "ing", "bri", "ng"}, atlas.MustBuild())
	})
	t.Run("4-value map[string]interface{str|int}", func(t *testing.T) {
		testRoundTripAllEncodings(t, map[string]interface{}{"k": "v", "a": "b", "z": 26, "m": 9}, atlas.MustBuild())
	})
	t.Run("cbor tagging and str-str transform", func(t *testing.T) {
		type Taggery string
		roundTrip(t,
			map[string]interface{}{"k": Taggery("v"), "a": "b", "z": 26, "m": 9},
			cbor.EncodeOptions{}, cbor.DecodeOptions{},
			atlas.MustBuild(
				atlas.BuildEntry(Taggery("")).UseTag(54).Transform().
					TransformMarshal(atlas.MakeMarshalTransformFunc(
						func(x Taggery) (string, error) {
							return string(x), nil
						})).
					TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
						func(x string) (Taggery, error) {
							return Taggery(x), nil
						})).
					Complete()),
		)
	})
	t.Run("cbor tagging and struct-[]byte transform", func(t *testing.T) {
		type Taggery struct{ x []byte }
		roundTrip(t,
			map[string]interface{}{
				"foo":   "bar",
				"hello": Taggery{[]byte("c1")},
				"baz": []interface{}{
					Taggery{[]byte("c1")},
					Taggery{[]byte("c2")},
				},
				"cats": map[string]interface{}{
					"qux": Taggery{[]byte("c3")},
				},
			},
			cbor.EncodeOptions{}, cbor.DecodeOptions{},
			atlas.MustBuild(
				atlas.BuildEntry(Taggery{}).UseTag(54).Transform().
					TransformMarshal(atlas.MakeMarshalTransformFunc(
						func(x Taggery) ([]byte, error) {
							return x.x, nil
						})).
					TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
						func(x []byte) (Taggery, error) {
							return Taggery{x}, nil
						})).
					Complete()),
		)
	})
}

func testRoundTripAllEncodings(
	t *testing.T,
	value interface{},
	atl atlas.Atlas,
) {
	t.Run("cbor", func(t *testing.T) {
		roundTrip(t, value, cbor.EncodeOptions{}, cbor.DecodeOptions{}, atl)
	})
	t.Run("json", func(t *testing.T) {
		roundTrip(t, value, json.EncodeOptions{}, json.DecodeOptions{}, atl)
	})
}

func roundTrip(
	t *testing.T,
	value interface{},
	encodeOptions refmt.EncodeOptions,
	decodeOptions refmt.DecodeOptions,
	atl atlas.Atlas,
) {
	// Encode.
	var buf bytes.Buffer
	encoder := refmt.NewMarshallerAtlased(encodeOptions, &buf, atl)
	if err := encoder.Marshal(value); err != nil {
		t.Fatalf("failed encoding: %s", err)
	}

	// Decode back to obj.
	decoder := refmt.NewUnmarshallerAtlased(decodeOptions, bytes.NewBuffer(buf.Bytes()), atl)
	var slot interface{}
	if err := decoder.Unmarshal(&slot); err != nil {
		t.Fatalf("failed decoding: %s", err)
	}
	t.Logf("%T -- %#v", slot, slot)

	// Re-encode.  Expect to get same encoded form.
	var buf2 bytes.Buffer
	encoder2 := refmt.NewMarshallerAtlased(encodeOptions, &buf2, atl)
	if err := encoder2.Marshal(slot); err != nil {
		t.Fatalf("failed re-encoding: %s", err)
	}

	// Stringify.  (Plain "%q" escapes unprintables quite nicely.)
	str1 := fmt.Sprintf("%q", buf.String())
	str2 := fmt.Sprintf("%q", buf2.String())
	if str1 != str2 {
		t.Errorf("%q != %q", str1, str2)
	}
	t.Logf("%#v == %q", value, str1)
}
