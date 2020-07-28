// +build !go1.13

package internal

import "golang.org/x/xerrors"

// Errorf wraps xerrors.Errorf.
func Errorf(format string, a ...interface{}) error { return xerrors.Errorf(format, a...) }

// ErrorAs wraps xerrors.As.
func ErrorAs(err error, target interface{}) bool { return xerrors.As(err, target) }

// ErrorIs wraps xerrors.Is.
func ErrorIs(err, target error) bool { return xerrors.Is(err, target) }

// NewError wraps xerrors.New.
func NewError(text string) error { return xerrors.New(text) }
