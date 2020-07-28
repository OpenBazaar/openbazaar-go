package wish

import (
	"reflect"

	"github.com/warpfork/go-wish/cmp"
)

var (
	_ Checker = ShouldBe
	_ Checker = ShouldEqual
)

// ShouldBe asserts that two values are *exactly* the same.
//
// In almost all cases, prefer ShouldEqual.
// ShouldBe differs from ShouldEqual in that it does *not* recurse, and
// thus can be used to explicitly check pointer equality.
//
// For pointers, ShouldBe checks pointer equality.
// Using ShouldBe on any kind of values which require recursion to meaningfully
// compare (e.g., structs, maps, arrays) will be rejected, as will using
// ShouldBe on any kind of value which is already never recursive (e.g. any
// primitives) since you can already use ShouldEqual to compare these.
func ShouldBe(actual interface{}, desire interface{}) (problem string, passed bool) {
	switch reflect.TypeOf(desire).Kind() {
	case // primitives
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Bool, reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.UnsafePointer:
		panic("use ShouldEqual instead of ShouldBe when comparing primitives")
	case // recursives
		reflect.Interface, reflect.Array, reflect.Map, reflect.Slice, reflect.Struct:
		panic("use ShouldEqual instead of ShouldBe when comparing recursive values")
	case reflect.Ptr:
		panic("TODO")
	case reflect.Func:
		panic("TODO")
	case reflect.Chan:
		panic("TODO")
	default:
		panic("unknown kind")
	}
}

// ShouldEqual asserts that two values are the same, examining the values
// recursively as necessary.  Maps, slices, and structs are all
// valid to compare with ShouldEqual.  Pointers will be traversed, and
// comparison continues with the values referenced by the pointer.
func ShouldEqual(actual interface{}, desire interface{}) (diff string, eq bool) {
	s1, ok1 := actual.(string)
	s2, ok2 := desire.(string)
	if ok1 && ok2 {
		diff = strdiff(s1, s2)
	} else {
		diff = cmp.Diff(actual, desire)
	}
	return diff, diff == ""
}
