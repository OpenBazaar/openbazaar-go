package model

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

// ToFloat ensures that any value returned by an insight or blockbook response is case to a float64 or errors
func ToFloat(i interface{}) (float64, error) {
	_, fok := i.(float64)
	_, sok := i.(string)
	if fok {
		return i.(float64), nil
	} else if sok {
		s := i.(string)
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("error parsing value float: %s", err)
		}
		return f, nil
	} else {
		return 0, errors.New("Unknown value type in response")
	}
}

// DefaultPort returns the default port for the given connection scheme unless
// otherwise indicated in the url.URL provided
func DefaultPort(u *url.URL) int {
	var port int
	if parsedPort, err := strconv.ParseInt(u.Port(), 10, 32); err == nil {
		port = int(parsedPort)
	}
	if port == 0 {
		if HasImpliedURLSecurity(u) {
			port = 443
		} else {
			port = 80
		}
	}
	return port
}

// HasImpliedURLSecurity returns true if the scheme is https
func HasImpliedURLSecurity(u *url.URL) bool { return u.Scheme == "https" }
