package internal

import (
	"encoding/base64"
	"encoding/json"
)

// Decode decodes a Base64 encoded JSON object using the proper encoding for JWTs.
func Decode(enc []byte, v interface{}) error {
	dec, err := DecodeToBytes(enc)
	if err != nil {
		return err
	}
	return json.Unmarshal(dec, v)
}

// DecodeToBytes decodes a Base64 string using the proper encoding for JWTs.
func DecodeToBytes(enc []byte) ([]byte, error) {
	encoding := base64.RawURLEncoding
	dec := make([]byte, encoding.DecodedLen(len(enc)))
	if _, err := encoding.Decode(dec, enc); err != nil {
		return nil, err
	}
	return dec, nil
}
