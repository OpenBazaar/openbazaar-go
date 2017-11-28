// cmd_protocolinfo.go - PROTOCOLINFO command.
//
// To the extent possible under law, Yawning Angel waived all copyright
// and related or neighboring rights to bulb, using the creative
// commons "cc0" public domain dedication. See LICENSE or
// <http://creativecommons.org/publicdomain/zero/1.0/> for full details.

package bulb

import (
	"strconv"
	"strings"

	"github.com/yawning/bulb/utils"
)

// ProtocolInfo is the result of the ProtocolInfo command.
type ProtocolInfo struct {
	AuthMethods map[string]bool
	CookieFile  string
	TorVersion  string

	RawResponse *Response
}

// ProtocolInfo issues a PROTOCOLINFO command and returns the parsed response.
func (c *Conn) ProtocolInfo() (*ProtocolInfo, error) {
	// In the pre-authentication state, only one PROTOCOLINFO command
	// may be issued.  Cache the value returned so that subsequent
	// calls continue to work.
	if !c.isAuthenticated && c.cachedPI != nil {
		return c.cachedPI, nil
	}

	resp, err := c.Request("PROTOCOLINFO")
	if err != nil {
		return nil, err
	}

	// Parse out the PIVERSION to make sure it speaks something we understand.
	if len(resp.Data) < 1 {
		return nil, newProtocolError("missing PIVERSION")
	}
	switch resp.Data[0] {
	case "1":
		return nil, newProtocolError("invalid PIVERSION: '%s'", resp.Reply)
	default:
	}

	// Parse out the rest of the lines.
	pi := new(ProtocolInfo)
	pi.RawResponse = resp
	pi.AuthMethods = make(map[string]bool)
	for i := 1; i < len(resp.Data); i++ {
		splitLine := utils.SplitQuoted(resp.Data[i], '"', ' ')
		switch splitLine[0] {
		case "AUTH":
			// Parse an AuthLine detailing how to authenticate.
			if len(splitLine) < 2 {
				continue
			}
			methods := strings.TrimPrefix(splitLine[1], "METHODS=")
			if methods == splitLine[1] {
				continue
			}
			for _, meth := range strings.Split(methods, ",") {
				pi.AuthMethods[meth] = true
			}

			if len(splitLine) < 3 {
				continue
			}
			cookiePath := strings.TrimPrefix(splitLine[2], "COOKIEFILE=")
			if cookiePath == splitLine[2] {
				continue
			}
			pi.CookieFile, _ = strconv.Unquote(cookiePath)
		case "VERSION":
			// Parse a VersionLine detailing the Tor version.
			if len(splitLine) < 2 {
				continue
			}
			torVersion := strings.TrimPrefix(splitLine[1], "Tor=")
			if torVersion == splitLine[1] {
				continue
			}
			pi.TorVersion, _ = strconv.Unquote(torVersion)
		default: // MUST ignore unsupported InfoLines.
		}
	}
	if !c.isAuthenticated {
		c.cachedPI = pi
	}
	return pi, nil
}
