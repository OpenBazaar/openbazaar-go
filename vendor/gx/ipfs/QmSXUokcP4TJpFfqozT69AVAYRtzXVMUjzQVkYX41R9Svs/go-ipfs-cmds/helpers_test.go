package cmds

import (
	"io"
	"testing"
)

// nopClose implements io.Close and does nothing
type nopCloser struct{}

func (c nopCloser) Close() error { return nil }

type testEmitter testing.T

func (s *testEmitter) Close() error {
	return nil
}

func (s *testEmitter) SetLength(_ uint64) {}
func (s *testEmitter) CloseWithError(err error) error {
	if err != nil {
		(*testing.T)(s).Error(err)
	}
	return nil
}
func (s *testEmitter) Emit(value interface{}) error {
	return nil
}

// newTestEmitter fails the test if it receives an error.
func newTestEmitter(t *testing.T) *testEmitter {
	return (*testEmitter)(t)
}

// noop does nothing and can be used as a noop Run function
func noop(req *Request, re ResponseEmitter, env Environment) error { return nil }

// writecloser implements io.WriteCloser by embedding
// an io.Writer and an io.Closer
type writecloser struct {
	io.Writer
	io.Closer
}
