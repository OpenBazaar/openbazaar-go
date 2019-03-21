package debug

import (
	"fmt"
	"runtime/debug"

	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

// UnexpectedError wraps a *cmdkit.Error that is being emitted. That type (and its non-pointer version) used to be emitted to indicate an error. That was deprecated in favour of CloseWithError.
type UnexpectedError struct {
	Err *cmdkit.Error
}

// Error returns the error string
func (err UnexpectedError) Error() string {
	return fmt.Sprintf("unexpected error value emitted: %q at\n%s", err.Err.Error(), debug.Stack())
}

// AssertNotError verifies that v is not a cmdkit.Error or *cmdkit.Error. Otherwise it panics.
func AssertNotError(v interface{}) {
	if e, ok := v.(cmdkit.Error); ok {
		v = &e
	}
	if e, ok := v.(*cmdkit.Error); ok {
		panic(UnexpectedError{e})
	}
}
