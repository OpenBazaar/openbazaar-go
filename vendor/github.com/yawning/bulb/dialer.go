// dialer.go - Tor backed proxy.Dialer.
//
// To the extent possible under law, Yawning Angel waived all copyright
// and related or neighboring rights to bulb, using the creative
// commons "cc0" public domain dedication. See LICENSE or
// <http://creativecommons.org/publicdomain/zero/1.0/> for full details.

package bulb

import (
	"strconv"
	"strings"

	"golang.org/x/net/proxy"
)

// Dialer returns a proxy.Dialer for the given Tor instance.
func (c *Conn) Dialer(auth *proxy.Auth) (proxy.Dialer, error) {
	const (
		cmdGetInfo     = "GETINFO"
		socksListeners = "net/listeners/socks"
		unixPrefix     = "unix:"
	)

	// Query for the SOCKS listeners via a GETINFO request.
	resp, err := c.Request("%s %s", cmdGetInfo, socksListeners)
	if err != nil {
		return nil, err
	}

	if len(resp.Data) != 1 {
		return nil, newProtocolError("no SOCKS listeners configured")
	}
	splitResp := strings.Split(resp.Data[0], " ")
	if len(splitResp) < 1 {
		return nil, newProtocolError("no SOCKS listeners configured")
	}

	// The first listener will have a "net/listeners/socks=" prefix, and all
	// entries are QuotedStrings.
	laddrStr := strings.TrimPrefix(splitResp[0], socksListeners+"=")
	if laddrStr == splitResp[0] {
		return nil, newProtocolError("failed to parse SOCKS listener")
	}
	laddrStr, _ = strconv.Unquote(laddrStr)

	// Construct the proxyDialer.
	if strings.HasPrefix(laddrStr, unixPrefix) {
		unixPath := strings.TrimPrefix(laddrStr, unixPrefix)
		return proxy.SOCKS5("unix", unixPath, auth, proxy.Direct)
	}

	return proxy.SOCKS5("tcp", laddrStr, auth, proxy.Direct)
}
