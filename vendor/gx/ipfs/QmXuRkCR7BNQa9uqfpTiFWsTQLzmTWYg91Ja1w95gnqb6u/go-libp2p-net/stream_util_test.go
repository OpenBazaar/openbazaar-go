package net_test

import (
	"errors"
	"io"
	"testing"
	"time"

	inet "gx/ipfs/QmXuRkCR7BNQa9uqfpTiFWsTQLzmTWYg91Ja1w95gnqb6u/go-libp2p-net"
)

var errCloseFailed = errors.New("close failed")
var errWriteFailed = errors.New("write failed")
var errReadFailed = errors.New("read failed")

type stream struct {
	inet.Stream

	data []byte

	failRead, failWrite, failClose bool

	reset bool
}

func (s *stream) Reset() error {
	s.reset = true
	return nil
}

func (s *stream) Close() error {
	if s.failClose {
		return errCloseFailed
	}
	return nil
}

func (s *stream) SetDeadline(t time.Time) error {
	s.SetReadDeadline(t)
	s.SetWriteDeadline(t)
	return nil
}

func (s *stream) SetReadDeadline(t time.Time) error {
	return nil
}

func (s *stream) SetWriteDeadline(t time.Time) error {
	return nil
}

func (s *stream) Write(b []byte) (int, error) {
	if s.failWrite {
		return 0, errWriteFailed
	}
	return len(b), nil
}

func (s *stream) Read(b []byte) (int, error) {
	var err error
	if s.failRead {
		err = errReadFailed
	}
	if len(s.data) == 0 {
		if err == nil {
			err = io.EOF
		}
		return 0, err
	}
	n := copy(b, s.data)
	s.data = s.data[n:]
	return n, err
}

func TestNormal(t *testing.T) {
	var s stream
	if err := inet.FullClose(&s); err != nil {
		t.Fatal(err)
	}
	if s.reset {
		t.Fatal("stream should not have been reset")
	}
}

func TestFailRead(t *testing.T) {
	var s stream
	s.failRead = true
	if inet.FullClose(&s) != errReadFailed {
		t.Fatal("expected read to fail with:", errReadFailed)
	}
	if !s.reset {
		t.Fatal("expected stream to be reset")
	}
}

func TestFailClose(t *testing.T) {
	var s stream
	s.failClose = true
	if inet.FullClose(&s) != errCloseFailed {
		t.Fatal("expected close to fail with:", errCloseFailed)
	}
	if !s.reset {
		t.Fatal("expected stream to be reset")
	}
}

func TestFailWrite(t *testing.T) {
	var s stream
	s.failWrite = true
	if err := inet.FullClose(&s); err != nil {
		t.Fatal(err)
	}
	if s.reset {
		t.Fatal("stream should not have been reset")
	}
}

func TestReadDataOne(t *testing.T) {
	var s stream
	s.data = []byte{0}
	if err := inet.FullClose(&s); err != inet.ErrExpectedEOF {
		t.Fatal("expected:", inet.ErrExpectedEOF)
	}
	if !s.reset {
		t.Fatal("stream have been reset")
	}
}

func TestReadDataMany(t *testing.T) {
	var s stream
	s.data = []byte{0, 1, 2, 3}
	if err := inet.FullClose(&s); err != inet.ErrExpectedEOF {
		t.Fatal("expected:", inet.ErrExpectedEOF)
	}
	if !s.reset {
		t.Fatal("stream have been reset")
	}
}

func TestReadDataError(t *testing.T) {
	var s stream
	s.data = []byte{0, 1, 2, 3}
	s.failRead = true
	if err := inet.FullClose(&s); err != inet.ErrExpectedEOF {
		t.Fatal("expected:", inet.ErrExpectedEOF)
	}
	if !s.reset {
		t.Fatal("stream have been reset")
	}
}
