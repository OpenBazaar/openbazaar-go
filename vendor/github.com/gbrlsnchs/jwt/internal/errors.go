// +build go1.13

package internal

import (
	"errors"
	"fmt"
)

// Errorf wraps fmt.Errorf.
func Errorf(format string, a ...interface{}) error { return fmt.Errorf(format, a...) }

// ErrorAs wraps errors.As.
func ErrorAs(err error, target interface{}) bool { return errors.As(err, target) }

// ErrorIs wraps errors.Is.
func ErrorIs(err, target error) bool { return errors.Is(err, target) }

// NewError wraps errors.New.
func NewError(text string) error { return errors.New(text) }
