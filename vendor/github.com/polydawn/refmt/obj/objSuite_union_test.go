package obj

import (
	"testing"

	"github.com/polydawn/refmt/obj/atlas"
	. "github.com/polydawn/refmt/tok"
)

func TestUnionHandling(t *testing.T) {
	t.Run("hello union keyed", func(t *testing.T) {
		type WowUnion interface{} // unions with marker methods are even better, but we can't use non-top-level types for those so most of the tests don't do it.
		type WowAlpha struct {
			A1 string
			A2 string
		}
		type WowBeta struct {
			B1 string
			B2 string
		}
		atl := atlas.MustBuild(
			atlas.BuildEntry((*WowUnion)(nil)).KeyedUnion().
				Of(map[string]*atlas.AtlasEntry{
					"alpha": atlas.BuildEntry(WowAlpha{}).StructMap().Autogenerate().Complete(),
					"beta":  atlas.BuildEntry(WowBeta{}).StructMap().Autogenerate().Complete(),
				}),
		)
		seq := []Token{
			{Type: TMapOpen, Length: 1},
			TokStr("alpha"), {Type: TMapOpen, Length: 2},
			/**/ TokStr("a1"), TokStr("v1"),
			/**/ TokStr("a2"), TokStr("v2"),
			/**/ {Type: TMapClose},
			{Type: TMapClose},
		}
		t.Run("marshal", func(t *testing.T) {
			var value WowUnion = WowAlpha{"v1", "v2"}
			checkMarshalling(t, atl, &value, seq, nil)
		})
		t.Run("unmarshal", func(t *testing.T) {
			var slot WowUnion
			var expect WowUnion = WowAlpha{"v1", "v2"}
			checkUnmarshalling(t, atl, &slot, seq, &expect, nil)
		})
		// TODO marshalling without the pointer grab *doesn't* work, because it doesn't get the interface info.
		//   this is inevitable.. but the error messages here need work, because it's extremely easy to typo or just not know about this detail of Go.
		//checkMarshalling(t, atl, value, seq, nil)
	})
}
