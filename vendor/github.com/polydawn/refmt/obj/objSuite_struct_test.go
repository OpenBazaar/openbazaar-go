package obj

import (
	"reflect"
	"testing"

	"github.com/polydawn/refmt/obj/atlas"
	"github.com/polydawn/refmt/tok/fixtures"
)

func TestStructHandling(t *testing.T) {
	t.Run("tokens for map with one string field", func(t *testing.T) {
		seq := fixtures.SequenceMap["single row map"].Tokens
		t.Run("prism to object with explicit atlas", func(t *testing.T) {
			type tObjStr struct {
				X string
			}
			atlas := atlas.MustBuild(
				atlas.BuildEntry(tObjStr{}).StructMap().
					AddField("X", atlas.StructMapEntry{SerialName: "key"}).
					Complete(),
			)
			t.Run("marshal", func(t *testing.T) {
				value := tObjStr{"value"}
				checkMarshalling(t, atlas, value, seq, nil)
				checkMarshalling(t, atlas, &value, seq, nil)
			})
			t.Run("unmarshal", func(t *testing.T) {
				slot := &tObjStr{}
				expect := &tObjStr{"value"}
				checkUnmarshalling(t, atlas, slot, seq, expect, nil)
			})
			t.Run("unmarshal overwriting", func(t *testing.T) {
				slot := &tObjStr{"should be overruled"}
				expect := &tObjStr{"value"}
				checkUnmarshalling(t, atlas, slot, seq, expect, nil)
			})
		})
		t.Run("prism to object with autogen atlasentry", func(t *testing.T) {
			type tObjStr struct {
				Key string // these key downcased by default in autogen
			}
			atlas := atlas.MustBuild(
				atlas.BuildEntry(tObjStr{}).StructMap().Autogenerate().Complete(),
			)
			t.Run("marshal", func(t *testing.T) {
				value := tObjStr{"value"}
				checkMarshalling(t, atlas, value, seq, nil)
				checkMarshalling(t, atlas, &value, seq, nil)
			})
			t.Run("unmarshal", func(t *testing.T) {
				slot := &tObjStr{}
				expect := &tObjStr{"value"}
				checkUnmarshalling(t, atlas, slot, seq, expect, nil)
			})
		})
		t.Run("prism to object with additional fields", func(t *testing.T) {
			type tObjStr struct {
				Key   string
				Spare string
			}
			atlas := atlas.MustBuild(
				atlas.BuildEntry(tObjStr{}).StructMap().Autogenerate().Complete(),
			)
			t.Run("unmarshal", func(t *testing.T) {
				slot := &tObjStr{}
				expect := &tObjStr{"value", ""}
				checkUnmarshalling(t, atlas, slot, seq, expect, nil)
			})
			t.Run("unmarshal overwriting", func(t *testing.T) {
				slot := &tObjStr{"should be overruled", "untouched"}
				expect := &tObjStr{"value", "untouched"}
				checkUnmarshalling(t, atlas, slot, seq, expect, nil)
			})
		})
		t.Run("prism to object with no matching fields", func(t *testing.T) {
			type tObjStr struct {
				Spare string
			}
			t.Run("unmarshal rejected", func(t *testing.T) {
				atlas := atlas.MustBuild(
					atlas.BuildEntry(tObjStr{}).StructMap().Autogenerate().Complete(),
				)
				slot := &tObjStr{}
				expect := &tObjStr{}
				seq := seq[:2]
				checkUnmarshalling(t, atlas, slot, seq, expect, ErrNoSuchField{"key", reflect.TypeOf(tObjStr{}).String()})
			})
			t.Run("with keyignore configured", func(t *testing.T) {
				atlas := atlas.MustBuild(
					atlas.BuildEntry(tObjStr{}).StructMap().
						Autogenerate().
						IgnoreKey("key").
						Complete(),
				)
				t.Run("unmarshal accepted", func(t *testing.T) {
					slot := &tObjStr{}
					expect := &tObjStr{}
					checkUnmarshalling(t, atlas, slot, seq, expect, nil)
				})
				t.Run("marshal not glitchy", func(t *testing.T) {
					seq := seq.Clone()
					seq[1].Str = "spare" // this atlas can't map the field as 'key' since we ignored that one.
					value := tObjStr{"value"}
					checkMarshalling(t, atlas, value, seq, nil)
					checkMarshalling(t, atlas, &value, seq, nil)
				})
			})
		})
	})
	t.Run("tokens for map with no fields", func(t *testing.T) {
		seq := fixtures.SequenceMap["empty map"].Tokens
		t.Run("prism to object with explicitly empty atlas", func(t *testing.T) {
			// I don't know why you'd want to do this, but it shouldn't crash.
			type tObjStr struct {
				X string
			}
			atlas := atlas.MustBuild(
				atlas.BuildEntry(tObjStr{}).StructMap().Complete(),
			)
			t.Run("marshal", func(t *testing.T) {
				value := tObjStr{"value"}
				checkMarshalling(t, atlas, value, seq, nil)
				checkMarshalling(t, atlas, &value, seq, nil)
			})
			t.Run("unmarshal", func(t *testing.T) {
				slot := &tObjStr{}
				expect := &tObjStr{}
				checkUnmarshalling(t, atlas, slot, seq, expect, nil)
			})
			t.Run("unmarshal overwriting", func(t *testing.T) {
				slot := &tObjStr{"should be overruled"}
				expect := &tObjStr{"should be overruled"}
				checkUnmarshalling(t, atlas, slot, seq, expect, nil)
			})
		})
		t.Run("prism to fieldless struct", func(t *testing.T) {
			type tEmpty struct{}
			atlas := atlas.MustBuild(
				atlas.BuildEntry(tEmpty{}).StructMap().Autogenerate().Complete(),
			)
			t.Run("marshal", func(t *testing.T) {
				value := tEmpty{}
				checkMarshalling(t, atlas, value, seq, nil)
				checkMarshalling(t, atlas, &value, seq, nil)
			})
			t.Run("unmarshal", func(t *testing.T) {
				slot := &tEmpty{}
				expect := &tEmpty{}
				checkUnmarshalling(t, atlas, slot, seq, expect, nil)
			})
		})
	})
	t.Run("tokens for map containing empty map", func(t *testing.T) {
		seq := fixtures.SequenceMap["empty map nested in map"].Tokens
		t.Run("deeper fieldless struct", func(t *testing.T) {
			type tEmpty struct{}
			type tFoo struct {
				K tEmpty
			}
			atlas := atlas.MustBuild(
				atlas.BuildEntry(tFoo{}).StructMap().Autogenerate().Complete(),
				atlas.BuildEntry(tEmpty{}).StructMap().Autogenerate().Complete(),
			)
			t.Run("marshal", func(t *testing.T) {
				value := tFoo{}
				checkMarshalling(t, atlas, value, seq, nil)
				checkMarshalling(t, atlas, &value, seq, nil)
			})
			t.Run("unmarshal", func(t *testing.T) {
				slot := &tFoo{}
				expect := &tFoo{}
				checkUnmarshalling(t, atlas, slot, seq, expect, nil)
			})
		})
		seq = fixtures.SequenceMap["nil nested in map"].Tokens
		t.Run("deeper fieldless struct as ptr", func(t *testing.T) {
			type tEmpty struct{}
			type tFoo struct {
				K *tEmpty
			}
			atlas := atlas.MustBuild(
				atlas.BuildEntry(tFoo{}).StructMap().Autogenerate().Complete(),
				atlas.BuildEntry(tEmpty{}).StructMap().Autogenerate().Complete(),
			)
			t.Run("marshal", func(t *testing.T) {
				value := tFoo{}
				checkMarshalling(t, atlas, value, seq, nil)
				checkMarshalling(t, atlas, &value, seq, nil)
			})
			t.Run("unmarshal", func(t *testing.T) {
				slot := &tFoo{}
				expect := &tFoo{}
				checkUnmarshalling(t, atlas, slot, seq, expect, nil)
			})
		})
	})
	t.Run("tokens for map containing jumbles of things", func(t *testing.T) {
		seq := fixtures.SequenceMap["jumbles nested in map"].Tokens
		t.Run("deeper fieldless struct as ptr", func(t *testing.T) {
			type tEmpty struct{}
			type tFoo struct {
				S string
				M tEmpty
				I int
				K *tEmpty
			}
			atlas := atlas.MustBuild(
				atlas.BuildEntry(tFoo{}).StructMap().Autogenerate().Complete(),
				atlas.BuildEntry(tEmpty{}).StructMap().Autogenerate().Complete(),
			)
			t.Run("marshal", func(t *testing.T) {
				value := tFoo{"foo", tEmpty{}, 42, nil}
				checkMarshalling(t, atlas, value, seq, nil)
				checkMarshalling(t, atlas, &value, seq, nil)
			})
			t.Run("unmarshal", func(t *testing.T) {
				slot := &tFoo{"foo", tEmpty{}, 42, nil}
				expect := &tFoo{"foo", tEmpty{}, 42, nil}
				checkUnmarshalling(t, atlas, slot, seq, expect, nil)
			})
		})
	})
}
