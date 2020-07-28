package refmt

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/polydawn/refmt/json"
	"github.com/polydawn/refmt/obj/atlas"
)

func TestUnmarshal(t *testing.T) {
	Convey("json", t, func() {
		Convey("string", func() {
			var slot string
			bs := []byte(`"str"`)
			err := Unmarshal(json.DecodeOptions{}, bs, &slot)
			So(err, ShouldBeNil)
		})
		Convey("map", func() {
			var slot map[string]string
			bs := []byte(`{"x":"1"}`)
			err := Unmarshal(json.DecodeOptions{}, bs, &slot)
			So(err, ShouldBeNil)
		})
	})
}

func TestUnmarshalAtlased(t *testing.T) {
	Convey("json", t, func() {
		Convey("obj", func() {
			type testObj struct {
				X string
				Y string
			}
			var slot testObj
			atl := atlas.MustBuild(
				atlas.BuildEntry(testObj{}).
					StructMap().Autogenerate().
					Complete(),
			)
			bs := []byte(`{"x":"1","y":"2"}`)
			err := UnmarshalAtlased(json.DecodeOptions{}, bs, &slot, atl)
			So(err, ShouldBeNil)
		})
	})
}
