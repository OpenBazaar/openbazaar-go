package commonatlases

import (
	"time"

	"github.com/polydawn/refmt/obj/atlas"
)

var Time_AsUnixInt = atlas.BuildEntry(time.Time{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(x time.Time) (int64, error) {
			return x.Unix(), nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x int64) (time.Time, error) {
			return time.Unix(x, 0).UTC(), nil
		})).
	Complete()

var Time_AsRFC3339 = atlas.BuildEntry(time.Time{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(x time.Time) (string, error) {
			return x.Format(time.RFC3339), nil
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x string) (time.Time, error) {
			return time.Parse(time.RFC3339, x)
		})).
	Complete()
