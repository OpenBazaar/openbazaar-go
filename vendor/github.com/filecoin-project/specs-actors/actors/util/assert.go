package util

import "fmt"

// Indicates a condition that should never happen. If encountered, execution will halt and the
// resulting state is undefined.
func AssertMsg(b bool, format string, a ...interface{}) {
	if !b {
		panic(fmt.Sprintf(format, a...))
	}
}

func Assert(b bool) {
	AssertMsg(b, "assertion failed")
}

func AssertNoError(e error) {
	if e != nil {
		panic(e.Error())
	}
}
