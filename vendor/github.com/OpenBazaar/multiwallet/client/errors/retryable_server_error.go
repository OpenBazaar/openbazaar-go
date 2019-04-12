package errors

import (
	"errors"
	"fmt"
)

// RetryableServerError is a special error interface which is understood by
// client/pool for the purposes of indicating whether the error should
// conditionally retry the request, which is indicated by IsRetryable
// returning true.
type RetryableServerError interface {
	wrappedError
	IsRetryable() bool
}

// RetryableErrorInstance is a simple type which implements the
// RetryableServerError interface. This pattern can be extended to create
// other complex error types which can also conditionally respond as
// retryable.
type RetryableErrorInstance struct {
	err error
}

// NewRetryableError is a helper that produces a RetryableServerError
func NewRetryableError(errReason string) RetryableServerError {
	return RetryableErrorInstance{err: errors.New(errReason)}
}

// NewRetryableErrorf is a helper that produces a RetryableServerError
func NewRetryableErrorf(format string, args ...interface{}) RetryableServerError {
	return RetryableErrorInstance{err: fmt.Errorf(format, args...)}
}

// MakeRetryable wraps an existing error into a RetryableServerError.
// This method can be used to wrap other already wrappedError types.
func MakeRetryable(err error) RetryableServerError {
	return RetryableErrorInstance{err: err}
}

// Error returns the error message
func (e RetryableErrorInstance) Error() string { return e.err.Error() }

func (e RetryableErrorInstance) internalError() error { return e.err }

// IsRetryable indicates whether the error should be attempted again
func (e RetryableErrorInstance) IsRetryable() bool { return true }
