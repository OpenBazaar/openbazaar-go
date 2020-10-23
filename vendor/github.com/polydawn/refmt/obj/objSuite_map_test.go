package obj

import (
	"testing"

	"github.com/polydawn/refmt/obj/atlas"
	"github.com/polydawn/refmt/tok/fixtures"
)

func TestMapHandling(t *testing.T) {
	t.Run("tokens for map with one string field", func(t *testing.T) {
		seq := fixtures.SequenceMap["single row map"].Tokens
		t.Run("prism to map[string]string", func(t *testing.T) {
			atlas := atlas.MustBuild()
			t.Run("marshal", func(t *testing.T) {
				value := map[string]string{"key": "value"}
				checkMarshalling(t, atlas, value, seq, nil)
				checkMarshalling(t, atlas, &value, seq, nil)
			})
			t.Run("unmarshal", func(t *testing.T) {
				slot := map[string]string{}
				expect := map[string]string{"key": "value"}
				checkUnmarshalling(t, atlas, &slot, seq, &expect, nil)
			})
		})
		t.Run("prism to map[TransformedStruct]string", func(t *testing.T) {
			type Keyish struct {
				Key string
			}
			atl := atlas.MustBuild(
				atlas.BuildEntry(Keyish{}).Transform().
					TransformMarshal(atlas.MakeMarshalTransformFunc(
						func(x Keyish) (string, error) {
							return x.Key, nil
						})).
					TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
						func(x string) (Keyish, error) {
							return Keyish{x}, nil
						})).
					Complete(),
			)
			t.Run("marshal", func(t *testing.T) {
				value := map[Keyish]string{{"key"}: "value"}
				checkMarshalling(t, atl, value, seq, nil)
				checkMarshalling(t, atl, &value, seq, nil)
			})
			t.Run("unmarshal", func(t *testing.T) {
				slot := map[Keyish]string{}
				expect := map[Keyish]string{{"key"}: "value"}
				checkUnmarshalling(t, atl, &slot, seq, &expect, nil)
			})
		})
	})
}
