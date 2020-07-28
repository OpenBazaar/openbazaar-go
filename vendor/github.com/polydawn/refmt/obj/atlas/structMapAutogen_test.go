package atlas

import (
	"encoding/json"
	"reflect"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStructMapAutogen(t *testing.T) {

	Convey("StructMap Autogen:", t, func() {
		type BB struct {
			Z string
		}
		type AA struct {
			X string
			Y BB
		}
		Convey("for a type which references other types, but is flat", func() {
			Convey("autogen works", func() {
				entry := AutogenerateStructMapEntry(reflect.TypeOf(AA{}))
				So(len(entry.StructMap.Fields), ShouldEqual, 2)
				So(entry.StructMap.Fields[0].SerialName, ShouldEqual, "x")
				So(entry.StructMap.Fields[0].ReflectRoute, ShouldResemble, ReflectRoute{0})
				So(entry.StructMap.Fields[0].Type, ShouldEqual, reflect.TypeOf(""))
				So(entry.StructMap.Fields[0].OmitEmpty, ShouldEqual, false)
				So(entry.StructMap.Fields[1].SerialName, ShouldEqual, "y")
				So(entry.StructMap.Fields[1].ReflectRoute, ShouldResemble, ReflectRoute{1})
				So(entry.StructMap.Fields[1].Type, ShouldEqual, reflect.TypeOf(BB{}))
				So(entry.StructMap.Fields[1].OmitEmpty, ShouldEqual, false)
			})
		})

		type CC struct {
			A AA
			BB
		}
		Convey("for a type which has some embedded structs", func() {
			Convey("sanity check: stdlib json sees this how we expect", func() {
				msg, err := json.Marshal(CC{AA{"a", BB{"z"}}, BB{"z2"}})
				So(err, ShouldBeNil)
				So(string(msg), ShouldResemble, `{"A":{"X":"a","Y":{"Z":"z"}},"Z":"z2"}`)
			})
			Convey("autogen works", func() {
				entry := AutogenerateStructMapEntry(reflect.TypeOf(CC{}))
				So(len(entry.StructMap.Fields), ShouldEqual, 2)
				So(entry.StructMap.Fields[0].SerialName, ShouldEqual, "a")
				So(entry.StructMap.Fields[0].ReflectRoute, ShouldResemble, ReflectRoute{0})
				So(entry.StructMap.Fields[0].Type, ShouldEqual, reflect.TypeOf(AA{}))
				So(entry.StructMap.Fields[0].OmitEmpty, ShouldEqual, false)
				So(entry.StructMap.Fields[1].SerialName, ShouldEqual, "z") // dives straight through embed!
				So(entry.StructMap.Fields[1].ReflectRoute, ShouldResemble, ReflectRoute{1, 0})
				So(entry.StructMap.Fields[1].Type, ShouldEqual, reflect.TypeOf(""))
				So(entry.StructMap.Fields[1].OmitEmpty, ShouldEqual, false)
			})
		})

		type DD struct {
			A   AA `                      refmt:"ooh"`
			BB  `       json:"bb"         refmt:"bb"`
			Non string `json:"-"          refmt:"-"`
			Om  string `json:",omitempty" refmt:",omitempty"`
		}
		// Interesting things to note:
		// - tagging an embedded field undoes the usual behavior of inlining it.
		Convey("for a type which is tagged and has some embedded structs", func() {
			Convey("sanity check: stdlib json sees this how we expect", func() {
				msg, err := json.Marshal(DD{AA{"a", BB{"z"}}, BB{"z2"}, "", ""})
				So(err, ShouldBeNil)
				So(string(msg), ShouldResemble, `{"A":{"X":"a","Y":{"Z":"z"}},"bb":{"Z":"z2"}}`)
			})
			Convey("autogen works", func() {
				entry := AutogenerateStructMapEntry(reflect.TypeOf(DD{}))
				So(len(entry.StructMap.Fields), ShouldEqual, 3)
				So(entry.StructMap.Fields[0].SerialName, ShouldEqual, "ooh")
				So(entry.StructMap.Fields[0].ReflectRoute, ShouldResemble, ReflectRoute{0})
				So(entry.StructMap.Fields[0].Type, ShouldEqual, reflect.TypeOf(AA{}))
				So(entry.StructMap.Fields[0].OmitEmpty, ShouldEqual, false)
				So(entry.StructMap.Fields[1].SerialName, ShouldEqual, "bb")
				So(entry.StructMap.Fields[1].ReflectRoute, ShouldResemble, ReflectRoute{1})
				So(entry.StructMap.Fields[1].Type, ShouldEqual, reflect.TypeOf(BB{}))
				So(entry.StructMap.Fields[1].OmitEmpty, ShouldEqual, false)
				So(entry.StructMap.Fields[2].SerialName, ShouldEqual, "om")
				So(entry.StructMap.Fields[2].ReflectRoute, ShouldResemble, ReflectRoute{3})
				So(entry.StructMap.Fields[2].Type, ShouldEqual, reflect.TypeOf(""))
				So(entry.StructMap.Fields[2].OmitEmpty, ShouldEqual, true)
			})
		})

		type EE struct {
			A **AA
			*BB
		}
		Convey("for a type which contains some pointer fields", func() {
			Convey("sanity check: stdlib json sees this how we expect", func() {
				aap := &AA{"a", BB{"z"}}
				msg, err := json.Marshal(EE{&aap, &BB{"z2"}})
				So(err, ShouldBeNil)
				So(string(msg), ShouldResemble, `{"A":{"X":"a","Y":{"Z":"z"}},"Z":"z2"}`)
			})
			Convey("autogen works", func() {
				entry := AutogenerateStructMapEntry(reflect.TypeOf(EE{}))
				So(len(entry.StructMap.Fields), ShouldEqual, 2)
				So(entry.StructMap.Fields[0].SerialName, ShouldEqual, "a")
				So(entry.StructMap.Fields[0].ReflectRoute, ShouldResemble, ReflectRoute{0})
				So(entry.StructMap.Fields[0].Type, ShouldEqual, reflect.PtrTo(reflect.TypeOf(&AA{})))
				So(entry.StructMap.Fields[0].OmitEmpty, ShouldEqual, false)
				So(entry.StructMap.Fields[1].SerialName, ShouldEqual, "z") // dives straight through embed!
				So(entry.StructMap.Fields[1].ReflectRoute, ShouldResemble, ReflectRoute{1, 0})
				So(entry.StructMap.Fields[1].Type, ShouldEqual, reflect.TypeOf(""))
				So(entry.StructMap.Fields[1].OmitEmpty, ShouldEqual, false)
			})
		})
	})
}
