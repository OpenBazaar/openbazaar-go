package cmds

import (
	"fmt"
	"io"

	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

var (
	ErrRcvdError = fmt.Errorf("received command error")
)

// Response is the result of a command request. Response is returned to the client.
type Response interface {
	Request() *Request

	Error() *cmdkit.Error
	Length() uint64

	// Next returns the next emitted value.
	// The returned error can be a network or decoding error.
	// The error can also be ErrRcvdError if an error has been emitted.
	// In this case the emitted error can be accessed using the Error() method.
	Next() (interface{}, error)
	RawNext() (interface{}, error)
}

type Head struct {
	Len uint64
	Err *cmdkit.Error
}

func (h Head) Length() uint64 {
	return h.Len
}

func (h Head) Error() *cmdkit.Error {
	return h.Err
}

// HandleError handles the error from cmds.Response.Next(), it returns
// true if Next() should be called again
func HandleError(err error, res Response, re ResponseEmitter) bool {
	if err != nil {
		if err == io.EOF {
			return false
		}

		if err == ErrRcvdError {
			err = res.Error()
		}

		if e, ok := err.(*cmdkit.Error); ok {
			re.SetError(e.Message, e.Code)
		} else {
			re.SetError(err, cmdkit.ErrNormal)
		}
		return false
	}
	return true
}
