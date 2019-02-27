package transport

import (
	"net/http"
	"time"
)

/**
End-point connection for given transport
*/
type Connection interface {
	/**
	Receive one more message, block until received
	*/
	GetMessage() (message string, err error)

	/**
	Send given message, block until sent
	*/
	WriteMessage(message string) error

	/**
	Close current connection
	*/
	Close()

	/**
	Get ping time interval and ping request timeout
	*/
	PingParams() (interval, timeout time.Duration)
}

/**
Connection factory for given transport
*/
type Transport interface {
	/**
	Get client connection
	*/
	Connect(url string) (conn Connection, err error)

	/**
	Handle one server connection
	*/
	HandleConnection(w http.ResponseWriter, r *http.Request) (conn Connection, err error)

	/**
	Serve HTTP request after making connection and events setup
	*/
	Serve(w http.ResponseWriter, r *http.Request)
}
