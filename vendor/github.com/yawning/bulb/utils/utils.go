// utils.go - A grab bag of useful utilitiy functions.
//
// To the extent possible under law, Yawning Angel waived all copyright
// and related or neighboring rights to bulb, using the creative
// commons "cc0" public domain dedication. See LICENSE or
// <http://creativecommons.org/publicdomain/zero/1.0/> for full details.

// Package utils implements useful utilities for dealing with Tor and it's
// control port.
package utils

import (
	"net"
	"net/url"
	"strconv"
)

// SplitQuoted splits s by sep if it is found outside substring
// quoted by quote.
func SplitQuoted(s string, quote, sep rune) (splitted []string) {
        quoteFlag := false
NewSubstring:
        for i, c := range s {
                if c == quote {
                        quoteFlag = !quoteFlag
                }
                if c == sep && !quoteFlag {
                        splitted = append(splitted, s[:i])
                        s = s[i+1:]
                        goto NewSubstring
                }
        }
        return append(splitted, s)
}

// ParseControlPortString parses a string representation of a control port
// address into a network/address string pair suitable for use with "dial".
//
// Valid string representations are:
//  * tcp://address:port
//  * unix://path
//  * port (Translates to tcp://127.0.0.1:port)
func ParseControlPortString(raw string) (network, addr string, err error) {
	// Try parsing it as a naked port.
	if _, err = strconv.ParseUint(raw, 10, 16); err == nil {
		raw = "tcp://127.0.0.1:" + raw
	}

	// Ok, parse/validate the URI.
	uri, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}
	if uri.Opaque != "" || uri.RawQuery != "" || uri.Fragment != "" {
		return "", "", net.InvalidAddrError("uri has Opaque/Query/Fragment")
	}
	switch uri.Scheme {
	case "tcp":
		if uri.Path != "" {
			return "", "", net.InvalidAddrError("tcp uri has a path")
		}
		tcpAddr, err := net.ResolveTCPAddr(uri.Scheme, uri.Host)
		if err != nil {
			return "", "", err
		}
		if tcpAddr.Port == 0 {
			return "", "", net.InvalidAddrError("tcp uri is missing a port")
		}
		return uri.Scheme, uri.Host, nil
	case "unix":
		if uri.Host != "" {
			return "", "", net.InvalidAddrError("unix uri has a host")
		}
		_, err := net.ResolveUnixAddr(uri.Scheme, uri.Path)
		if err != nil {
			return "", "", err
		}
		return uri.Scheme, uri.Path, nil
	}
	return "", "", net.InvalidAddrError("unknown scheme: " + uri.Scheme)
}
