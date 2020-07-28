//+build cgo

package main

import (
	"fmt"
	"os"

	ffi "github.com/filecoin-project/filecoin-ffi"
)

func main() {
	os.Setenv("RUST_LOG", "info")
	th := panicOnFailureTestHelper{}
	ffi.WorkflowGetGPUDevicesDoesNotProduceAnError(&th)
	ffi.WorkflowProofsLifecycle(&th)
	ffi.WorkflowRegisteredPoStProofFunctions(&th)
	ffi.WorkflowRegisteredSealProofFunctions(&th)
}

type panicOnFailureTestHelper struct{}

func (p panicOnFailureTestHelper) AssertEqual(expected, actual interface{}, msgAndArgs ...interface{}) bool {
	if expected != actual {
		panic(fmt.Sprintf("not equal: %+v, %+v, %+v", expected, actual, msgAndArgs))
	}

	return true
}

func (p panicOnFailureTestHelper) AssertNoError(err error, msgAndArgs ...interface{}) bool {
	if err != nil {
		panic(fmt.Sprintf("there was an error: %+v, %+v", err, msgAndArgs))
	}

	return true
}

func (p panicOnFailureTestHelper) AssertTrue(value bool, msgAndArgs ...interface{}) bool {
	if !value {
		panic(fmt.Sprintf("not true: %+v, %+v", value, msgAndArgs))
	}

	return true
}

func (p panicOnFailureTestHelper) RequireEqual(expected interface{}, actual interface{}, msgAndArgs ...interface{}) {
	if expected != actual {
		panic(fmt.Sprintf("not equal: %+v, %+v, %+v", expected, actual, msgAndArgs))
	}
}

func (p panicOnFailureTestHelper) RequireNoError(err error, msgAndArgs ...interface{}) {
	if err != nil {
		panic(fmt.Sprintf("there was an error: %+v, %+v", err, msgAndArgs))
	}
}

func (p panicOnFailureTestHelper) RequireTrue(value bool, msgAndArgs ...interface{}) {
	if !value {
		panic(fmt.Sprintf("not true: %+v, %+v", value, msgAndArgs))
	}
}
