package wish

import (
	"fmt"
)

// T is an interface alternative to `*testing.T` -- wherever you see this used,
// use your `*testing.T` object.
type T interface {
	Helper()
	Fail()
	FailNow()
	SkipNow()
	Log(...interface{})
	Name() string

	// Note the lack of `t.Run` in this interface.  Two reasons:
	//  - wish never launches sub-tests; that's for *you* to do (per "library, not framework");
	//  - we... can't really make it useful.  Stdlib's `Run` takes a *concrete* `*testing.T`.
}

// Checker functions compare two objects, report if they "match" (the semantics of
// which should be documented by the Checker function), and if the objects do not
// "match", should provide a descriptive message of how the objects mismatch.
type Checker func(actual interface{}, desire interface{}) (problem string, passed bool)

// Wish makes an assertion that two objects match, using criteria defined by a
// Checker function, and will log information about this to the given T.
//
// Failure to match will log to T, fail the test, and return false (so you
// may take alternative debugging paths, or handle halting on your own).
// Failure to match will *not* cause FailNow; execution will continue.
func Wish(t T, actual interface{}, check Checker, desired interface{}, opts ...options) bool {
	t.Helper()
	problemMsg, passed := check(actual, desired)
	if !passed {
		t.Log(fmt.Sprintf("%s check rejected:\n%s", getCheckerShortName(check), Indent(problemMsg)))
		t.Fail()
	}
	return passed
}

// Require makes an assertion that two objects match, using criteria defined by a
// Checker function, and will halt execution via T.FailNow in case of failure.
//
// Failure to match will log to T in the same way as Wish, and call T.FailNow,
// which will halt execution immediately similar to a panic.
//
// Require can be helpful to divert control flow, usually to avoid making
// checks that may raise panics if the Require check fails (e.g. checking
// the a value is non-nil and halting before dereferencing them).
// However, always consider if you can simply use Wish instead.
// For example, the following is an unnecessary use of Require:
//
// 	Require(err, ShouldNotEqual, nil)
// 	Wish(err.Error(), ShouldEqual, "msg")
//
// It's a natural instinct to write checks in two parts like this when
// accustomed to the standard library tools, but we can do better.
// Instead, accomplish both checks at once, like this:
//
// 	Wish(err, ShouldEqual, fmt.Errorf("msg"))
//
// Since ShouldEqual already gracefully handles nil values, as well as
// comparing the types recursively via reflection, this is usage is correct.
// It's also shorter to type.  Most importantly, this approach produces more
// useful rejection messages, because it can say what you *do* expect, rather
// than halting after a less informative check.
func Require(t T, actual interface{}, check Checker, desired interface{}, opts ...options) {
	t.Helper()
	problemMsg, passed := check(actual, desired)
	if !passed {
		t.Log(fmt.Sprintf("halting: critical %s check rejected:\n%s", getCheckerShortName(check), Indent(problemMsg)))
		t.FailNow()
	}
}

type options interface {
	_options()
}
