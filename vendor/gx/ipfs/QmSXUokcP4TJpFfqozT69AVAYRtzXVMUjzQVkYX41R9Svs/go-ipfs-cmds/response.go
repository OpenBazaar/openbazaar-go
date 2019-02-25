package cmds

import (
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

// Response is the result of a command request. Response is returned to the client.
type Response interface {
	Request() *Request

	Error() *cmdkit.Error
	Length() uint64

	// Next returns the next emitted value.
	// The returned error can be a network or decoding error.
	Next() (interface{}, error)
}
