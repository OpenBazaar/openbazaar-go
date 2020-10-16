package exitcode

import (
	"errors"
	"fmt"
	"strconv"

	"golang.org/x/xerrors"
)

type ExitCode int64

func (x ExitCode) IsSuccess() bool {
	return x == Ok
}

func (x ExitCode) IsError() bool {
	return !x.IsSuccess()
}

// Whether an exit code indicates a message send failure.
// A send failure means that the caller's CallSeqNum is not incremented and the caller has not paid
// gas fees for the message (because the caller doesn't exist or can't afford it).
// A receipt with send failure does not indicate that the message (or another one carrying the same CallSeqNum)
// could not apply in the future, against a different state.
func (x ExitCode) IsSendFailure() bool {
	return x == SysErrSenderInvalid || x == SysErrSenderStateInvalid
}

// A non-canonical string representation for human inspection.
func (x ExitCode) String() string {
	name, ok := names[x]
	if ok {
		return fmt.Sprintf("%s(%d)", name, x)
	}
	return strconv.FormatInt(int64(x), 10)
}

// Implement error to trigger Go compiler checking of exit code return values.
func (x ExitCode) Error() string {
	return x.String()
}

// Wrapf attaches an error message, and possibly an error, to the exit
// code.
//
//    err := ErrIllegalArgument.Wrapf("my description: %w", err)
//    exitcode.Unwrap(exitcode.ErrIllegalState, err) == exitcode.ErrIllegalArgument
func (x ExitCode) Wrapf(msg string, args ...interface{}) error {
	return &wrapped{
		exitCode: x,
		cause:    xerrors.Errorf(msg, args...),
	}
}

type wrapped struct {
	exitCode ExitCode
	cause    error
}

func (w *wrapped) String() string {
	return w.Error()
}

func (w *wrapped) Error() string {
	// Don't include the exit code. That will be handled by the runtime and
	// this error has likely been wrapped multiple times.
	return w.cause.Error()
}

// implements the interface required by errors.As
func (w *wrapped) As(target interface{}) bool {
	return errors.As(w.exitCode, target) || errors.As(w.cause, target)
}

// implements the interface required by errors.Is
func (w *wrapped) Is(target error) bool {
	if _, ok := target.(ExitCode); ok {
		// If the target is an exit code, make sure we shadow lower exit
		// codes.
		return w.exitCode == target
	}
	return errors.Is(w.cause, target)
}

// Unwrap extracts an exit code from an error, defaulting to the passed default
// exit code.
//
//    err := ErrIllegalState.WithContext("my description: %w", err)
//    exitcode.Unwrap(exitcode.ErrIllegalState, err) == exitcode.ErrIllegalArgument
func Unwrap(err error, defaultExitCode ExitCode) (code ExitCode) {
	if errors.As(err, &code) {
		return code
	}
	return defaultExitCode
}
