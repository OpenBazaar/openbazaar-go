package cmdkit

import (
	"encoding/json"
	"errors"
)

// ErrorType signfies a category of errors
type ErrorType uint

// ErrorTypes convey what category of error ocurred
const (
	ErrNormal         ErrorType = iota // general errors
	ErrClient                          // error was caused by the client, (e.g. invalid CLI usage)
	ErrImplementation                  // programmer error in the server
	ErrNotFound                        // == HTTP 404
	ErrFatal                           // abort instantly
	// TODO: add more types of errors for better error-specific handling
)

// Error is a struct for marshalling errors
type Error struct {
	Message string
	Code    ErrorType
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Message string
		Code    ErrorType
		Type    string
	}{
		Message: e.Message,
		Code:    e.Code,
		Type:    "error",
	})
}

func (e *Error) UnmarshalJSON(data []byte) error {
	var w struct {
		Message string
		Code    ErrorType
		Type    string
	}

	err := json.Unmarshal(data, &w)
	if err != nil {
		return err
	}

	if w.Type != "error" {
		return errors.New("not of type error")
	}

	e.Message = w.Message
	e.Code = w.Code

	return nil
}
