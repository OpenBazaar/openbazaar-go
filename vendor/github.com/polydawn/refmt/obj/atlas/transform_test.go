package atlas

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type tObjStr struct {
	X string
}

func TestTransformBuilder(t *testing.T) {
	Convey("Building atlases using transforms:", t, func() {
		Convey("string->struct->string happy path should build without error", func() {
			_, err := Build(
				BuildEntry(tObjStr{}).Transform().
					TransformMarshal(MakeMarshalTransformFunc(
						func(x tObjStr) (string, error) {
							return x.X, nil
						})).
					TransformUnmarshal(MakeUnmarshalTransformFunc(
						func(x string) (tObjStr, error) {
							return tObjStr{x}, nil
						})).
					Complete(),
			)
			So(err, ShouldBeNil)
		})
	})
}
