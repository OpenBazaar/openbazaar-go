package wish_test

import (
	"fmt"

	"github.com/warpfork/go-wish"
)

// fakeT is a dummy replacement for `*testing.T` which simply always prints
// logs to stderr immediately.  We need to use this in `Example*` funcs
// because creating a usable `*testing.T` is impossible.
type fakeT struct{}

func (fakeT) Helper()              {}
func (fakeT) Fail()                {}
func (fakeT) FailNow()             {}
func (fakeT) SkipNow()             {}
func (fakeT) Log(x ...interface{}) { fmt.Print(x...) }
func (fakeT) Name() string         { return "testname" }

func ExampleThing() {
	t := &fakeT{}
	actual := "foobar"
	objective := "bazfomp"
	fmt.Printf("%v\n", wish.Wish(t, actual, wish.ShouldEqual, objective))

	// Output:
	// ShouldEqual check rejected:
	// 	@@ -1 +1 @@
	// 	- foobar
	// 	+ bazfomp
	// false
}

func ExampleMultilineString() {
	t := &fakeT{}
	actual := "foobar\nwoop\nwow"
	objective := "bazfomp\nwoop\nwowdiff"
	fmt.Printf("%v\n", wish.Wish(t, actual, wish.ShouldEqual, objective))

	// Output:
	// ShouldEqual check rejected:
	// 	@@ -1,3 +1,3 @@
	// 	- foobar\n
	// 	+ bazfomp\n
	// 	  woop\n
	// 	- wow
	// 	+ wowdiff
	// false
}

func ExampleWish_ShouldEqual_CompareStructs() {
	t := &fakeT{}
	actual := struct{ Baz string }{"asdf"}
	objective := struct{ Baz string }{"asdf"}
	fmt.Printf("%v\n", wish.Wish(t, actual, wish.ShouldEqual, objective))

	// Output:
	// true
}

func ExampleWish_ShouldEqual_CompareStructsReject() {
	t := &fakeT{}
	actual := struct{ Bar string }{"asdf"}
	objective := struct{ Baz string }{"qwer"}
	fmt.Printf("%v\n", wish.Wish(t, actual, wish.ShouldEqual, objective))

	// Output:
	// ShouldEqual check rejected:
	// 	  interface{}(
	// 	- 	struct{ Bar string }{Bar: "asdf"},
	// 	+ 	struct{ Baz string }{Baz: "qwer"},
	// 	  )
	// false
}

func ExampleWish_ShouldEqual_TypeMismatch() {
	t := &fakeT{}
	actual := "foobar"
	objective := struct{}{}
	fmt.Printf("%v\n", wish.Wish(t, actual, wish.ShouldEqual, objective))

	// Output:
	// ShouldEqual check rejected:
	// 	  interface{}(
	// 	- 	string("foobar"),
	// 	+ 	struct{}{},
	// 	  )
	// false
}
