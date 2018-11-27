package cli

import (
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"

	"gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds"
	"gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds/debug"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

var _ ResponseEmitter = &responseEmitter{}

func NewResponseEmitter(stdout, stderr io.Writer, req *cmds.Request) (cmds.ResponseEmitter, <-chan int, error) {
	ch := make(chan int)
	encType, enc, err := cmds.GetEncoder(req, stdout, cmds.TextNewline)
	if err != nil {
		close(ch)
		return nil, ch, err
	}

	return &responseEmitter{
		stdout:  stdout,
		stderr:  stderr,
		encType: encType,
		enc:     enc,
		ch:      ch,
	}, ch, err
}

// ResponseEmitter extends cmds.ResponseEmitter to give better control over the command line
type ResponseEmitter interface {
	cmds.ResponseEmitter

	Stdout() io.Writer
	Stderr() io.Writer
	Exit(int)
}

type responseEmitter struct {
	l      sync.Mutex
	stdout io.Writer
	stderr io.Writer

	length  uint64
	enc     cmds.Encoder
	encType cmds.EncodingType
	exit    int
	closed  bool

	ch chan<- int
}

func (re *responseEmitter) Type() cmds.PostRunType {
	return cmds.CLI
}

func (re *responseEmitter) SetLength(l uint64) {
	re.length = l
}

func (re *responseEmitter) CloseWithError(err error) error {
	if err == nil {
		return re.Close()
	}

	if e, ok := err.(cmdkit.Error); ok {
		err = &e
	}

	e, ok := err.(*cmdkit.Error)
	if !ok {
		e = &cmdkit.Error{
			Message: err.Error(),
		}
	}

	re.l.Lock()
	defer re.l.Unlock()

	if re.closed {
		return cmds.ErrClosingClosedEmitter
	}

	re.exit = 1 // TODO we could let err carry an exit code

	_, err = fmt.Fprintln(re.stderr, "Error:", e.Message)
	if err != nil {
		return err
	}

	return re.close()
}

func (re *responseEmitter) isClosed() bool {
	re.l.Lock()
	defer re.l.Unlock()

	return re.closed
}

func (re *responseEmitter) Close() error {
	re.l.Lock()
	defer re.l.Unlock()

	return re.close()
}

func (re *responseEmitter) close() error {
	if re.closed {
		return cmds.ErrClosingClosedEmitter
	}

	re.ch <- re.exit
	close(re.ch)

	defer func() {
		re.stdout = nil
		re.stderr = nil
		re.closed = true
	}()

	// ignore error if the operating system doesn't support syncing std{out,err}
	ignoreError := func(err error) bool {
		if perr, ok := err.(*os.PathError); ok &&
			perr.Op == "sync" && (perr.Err == syscall.EINVAL ||
			perr.Err == syscall.ENOTSUP) {
			return true
		}

		return false
	}

	if f, ok := re.stderr.(*os.File); ok {
		err := f.Sync()
		if err != nil {
			if !ignoreError(err) {
				return err
			}
		}
	}
	if f, ok := re.stdout.(*os.File); ok {
		err := f.Sync()
		if err != nil {
			if !ignoreError(err) {
				return err
			}
		}
	}

	return nil
}

func (re *responseEmitter) Emit(v interface{}) error {
	var isSingle bool
	// unwrap
	if val, ok := v.(cmds.Single); ok {
		v = val.Value
		isSingle = true
	}

	// Initially this library allowed commands to return errors by sending an
	// error value along a stream. We removed that in favour of CloseWithError,
	// so we want to make sure we catch situations where some code still uses the
	// old error emitting semantics and _panic_ in those situations.
	debug.AssertNotError(v)

	// channel emission iteration
	if ch, ok := v.(chan interface{}); ok {
		v = (<-chan interface{})(ch)
	}
	if ch, isChan := v.(<-chan interface{}); isChan {
		return cmds.EmitChan(re, ch)
	}

	// TODO find a better solution for this.
	// Idea: use the actual cmd.Type and not *cmd.Type
	// would need to fix all commands though
	switch c := v.(type) {
	case *string:
		v = *c
	case *int:
		v = *c
	}

	if re.isClosed() {
		return cmds.ErrClosedEmitter
	}

	var err error

	switch t := v.(type) {
	case io.Reader:
		_, err = io.Copy(re.stdout, t)
		if err != nil {
			return err
		}
	default:
		if re.enc != nil {
			err = re.enc.Encode(v)
		} else {
			_, err = fmt.Fprintln(re.stdout, t)
		}
	}

	if isSingle {
		return re.CloseWithError(err)
	}

	return err
}

// Stderr returns the ResponseWriter's stderr
func (re *responseEmitter) Stderr() io.Writer {
	return re.stderr
}

// Stdout returns the ResponseWriter's stdout
func (re *responseEmitter) Stdout() io.Writer {
	return re.stdout
}

// Exit sends code to the channel that was returned by NewResponseEmitter, so main() can pass it to os.Exit()
func (re *responseEmitter) Exit(code int) {
	defer re.Close()

	re.l.Lock()
	defer re.l.Unlock()
	re.exit = code
}
