package errors

import (
	"errors"
	"fmt"
)

// FatalServerError is a special error interface which is understood by
// client/pool for the purposes of indicating the error should conditionally
// trigger a non-recoverable failure for that request, which is indicated
// by IsFatal returning true.
type FatalServerError interface {
	wrappedError
	IsFatal() bool
}

// FatalServerErrorInstance is a simple type which implements
// FatalServerError interface. This pattern can be extended to create
// other complex error types which can also conditionally respond as
// non-recoverable.
type FatalServerErrorInstance struct {
	err error
}

// NewFatalError is a helper that produces a FatalServerError
func NewFatalError(errReason string) FatalServerError {
	return FatalServerErrorInstance{err: errors.New(errReason)}
}

// NewFatalErrorf is a helper that produces a FatalServerError
func NewFatalErrorf(format string, args ...interface{}) FatalServerError {
	return FatalServerErrorInstance{err: fmt.Errorf(format, args...)}
}

// MakeFatal wraps an existing error into a FatalServerError. MakeFatal is
// composable with other wrappedError types.
func MakeFatal(err error) FatalServerError {
	return FatalServerErrorInstance{err: err}
}

// Error returns the error message
func (e FatalServerErrorInstance) Error() string { return e.err.Error() }

func (e FatalServerErrorInstance) internalError() error { return e.err }

// IsFatal indicates whether the error should be considered non-recoverable
// for that request
func (e FatalServerErrorInstance) IsFatal() bool { return true }
