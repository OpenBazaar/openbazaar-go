package jwt

import (
	"errors"

	"github.com/gbrlsnchs/jwt/v3/internal"
)

// ErrMalformed indicates a token doesn't have a valid format, as per the RFC 7519.
var ErrMalformed = errors.New("jwt: malformed token")

// RawToken is a representation of a parsed JWT string.
type RawToken struct {
	token      []byte
	sep1, sep2 int

	hd  Header
	alg Algorithm

	pl  *Payload
	vds []Validator
}

func (rt *RawToken) header() []byte        { return rt.token[:rt.sep1] }
func (rt *RawToken) headerPayload() []byte { return rt.token[:rt.sep2] }
func (rt *RawToken) payload() []byte       { return rt.token[rt.sep1+1 : rt.sep2] }
func (rt *RawToken) sig() []byte           { return rt.token[rt.sep2+1:] }

func (rt *RawToken) setToken(token []byte, sep1, sep2 int) {
	rt.sep1 = sep1
	rt.sep2 = sep1 + 1 + sep2
	rt.token = token
}

func (rt *RawToken) decode(payload interface{}) (err error) {
	if err = internal.Decode(rt.payload(), payload); err != nil {
		return err
	}
	for _, vd := range rt.vds {
		if err = vd(rt.pl); err != nil {
			return err
		}
	}
	return nil
}

func (rt *RawToken) decodeHeader() error {
	if err := internal.Decode(rt.header(), &rt.hd); err != nil {
		return err
	}
	return nil
}
