// conn.go - Controller connection instance.
//
// To the extent possible under law, Yawning Angel waived all copyright
// and related or neighboring rights to bulb, using the creative
// commons "cc0" public domain dedication. See LICENSE or
// <http://creativecommons.org/publicdomain/zero/1.0/> for full details.

// Package bulb is a Go language interface to a Tor control port.
package bulb

import (
	"errors"
	gofmt "fmt"
	"io"
	"log"
	"net"
	"net/textproto"
	"sync"
)

const (
	maxEventBacklog    = 16
	maxResponseBacklog = 16
)

// ErrNoAsyncReader is the error returned when the asynchronous event handling
// is requested, but the helper go routine has not been started.
var ErrNoAsyncReader = errors.New("event requested without an async reader")

// Conn is a control port connection instance.
type Conn struct {
	conn            *textproto.Conn
	isAuthenticated bool
	debugLog        bool
	cachedPI        *ProtocolInfo

	asyncReaderLock    sync.Mutex
	asyncReaderRunning bool
	eventChan          chan *Response
	respChan           chan *Response
	closeWg            sync.WaitGroup

	rdErrLock sync.Mutex
	rdErr     error
}

func (c *Conn) setRdErr(err error, force bool) {
	c.rdErrLock.Lock()
	defer c.rdErrLock.Unlock()
	if c.rdErr == nil || force {
		c.rdErr = err
	}
}

func (c *Conn) getRdErr() error {
	c.rdErrLock.Lock()
	defer c.rdErrLock.Unlock()
	return c.rdErr
}

func (c *Conn) isAsyncReaderRunning() bool {
	c.asyncReaderLock.Lock()
	defer c.asyncReaderLock.Unlock()
	return c.asyncReaderRunning
}

func (c *Conn) asyncReader() {
	for {
		resp, err := c.ReadResponse()
		if err != nil {
			c.setRdErr(err, false)
			break
		}
		if resp.IsAsync() {
			c.eventChan <- resp
		} else {
			c.respChan <- resp
		}
	}
	close(c.eventChan)
	close(c.respChan)
	c.closeWg.Done()

	// In theory, we would lock and set asyncReaderRunning to false here, but
	// once it's started, the only way it returns is if there is a catastrophic
	// failure, or a graceful shutdown.  Changing this will require redoing how
	// Close() works.
}

// Debug enables/disables debug logging of control port chatter.
func (c *Conn) Debug(enable bool) {
	c.debugLog = enable
}

// Close closes the connection.
func (c *Conn) Close() error {
	c.asyncReaderLock.Lock()
	defer c.asyncReaderLock.Unlock()

	err := c.conn.Close()
	if err != nil && c.asyncReaderRunning {
		c.closeWg.Wait()
	}
	c.setRdErr(io.ErrClosedPipe, true)
	return err
}

// StartAsyncReader starts the asynchronous reader go routine that allows
// asynchronous events to be handled.  It must not be called simultaniously
// with Read, Request, or ReadResponse or undefined behavior will occur.
func (c *Conn) StartAsyncReader() {
	c.asyncReaderLock.Lock()
	defer c.asyncReaderLock.Unlock()
	if c.asyncReaderRunning {
		return
	}

	// Allocate the channels and kick off the read worker.
	c.eventChan = make(chan *Response, maxEventBacklog)
	c.respChan = make(chan *Response, maxResponseBacklog)
	c.closeWg.Add(1)
	go c.asyncReader()
	c.asyncReaderRunning = true
}

// NextEvent returns the next asynchronous event received, blocking if
// neccecary.  In order to enable asynchronous event handling, StartAsyncReader
// must be called first.
func (c *Conn) NextEvent() (*Response, error) {
	if err := c.getRdErr(); err != nil {
		return nil, err
	}
	if !c.isAsyncReaderRunning() {
		return nil, ErrNoAsyncReader
	}

	resp, ok := <-c.eventChan
	if resp != nil {
		return resp, nil
	} else if !ok {
		return nil, io.ErrClosedPipe
	}
	panic("BUG: NextEvent() returned a nil response and error")
}

// Request sends a raw control port request and returns the response.
// If the async. reader is not currently running, events received while waiting
// for the response will be silently dropped.  Calling Request simultaniously
// with StartAsyncReader, Read, Write, or ReadResponse will lead to undefined
// behavior.
func (c *Conn) Request(fmt string, args ...interface{}) (*Response, error) {
	if err := c.getRdErr(); err != nil {
		return nil, err
	}
	asyncResp := c.isAsyncReaderRunning()

	if c.debugLog {
		log.Printf("C: %s", gofmt.Sprintf(fmt, args...))
	}

	id, err := c.conn.Cmd(fmt, args...)
	if err != nil {
		return nil, err
	}

	c.conn.StartResponse(id)
	defer c.conn.EndResponse(id)
	var resp *Response
	if asyncResp {
		var ok bool
		resp, ok = <-c.respChan
		if resp == nil && !ok {
			return nil, io.ErrClosedPipe
		}
	} else {
		// Event handing requires the asyncReader() goroutine, try to get a
		// response, while silently swallowing events.
		for resp == nil || resp.IsAsync() {
			resp, err = c.ReadResponse()
			if err != nil {
				return nil, err
			}
		}
	}
	if resp == nil {
		panic("BUG: Request() returned a nil response and error")
	}
	if resp.IsOk() {
		return resp, nil
	}
	return resp, resp.Err
}

// Read reads directly from the control port connection.  Mixing this call
// with Request, ReadResponse, or asynchronous events will lead to undefined
// behavior.
func (c *Conn) Read(p []byte) (int, error) {
	return c.conn.R.Read(p)
}

// Write writes directly from the control port connection.  Mixing this call
// with Request will lead to undefined behavior.
func (c *Conn) Write(p []byte) (int, error) {
	n, err := c.conn.W.Write(p)
	if err == nil {
		// If the write succeeds, but the flush fails, n will be incorrect...
		return n, c.conn.W.Flush()
	}
	return n, err
}

// Dial connects to a given network/address and returns a new Conn for the
// connection.
func Dial(network, addr string) (*Conn, error) {
	c, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	return NewConn(c), nil
}

// NewConn returns a new Conn using c for I/O.
func NewConn(c io.ReadWriteCloser) *Conn {
	conn := new(Conn)
	conn.conn = textproto.NewConn(c)
	return conn
}

func newProtocolError(fmt string, args ...interface{}) textproto.ProtocolError {
	return textproto.ProtocolError(gofmt.Sprintf(fmt, args...))
}

var _ io.ReadWriteCloser = (*Conn)(nil)
