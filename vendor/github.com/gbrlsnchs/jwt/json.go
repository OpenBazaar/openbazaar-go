package jwt

import (
	"bytes"
	"errors"
)

// ErrNotJSONObject is the error for when a JWT payload is not a JSON object.
var ErrNotJSONObject = errors.New("jwt: payload is not a valid JSON object")

func isJSONObject(payload []byte) bool {
	payload = bytes.TrimSpace(payload)
	return payload[0] == '{' && payload[len(payload)-1] == '}'
}
