// cmd_authenticate.go - AUTHENTICATE/AUTHCHALLENGE commands.
//
// To the extent possible under law, Yawning Angel waived all copyright
// and related or neighboring rights to bulb, using the creative
// commons "cc0" public domain dedication. See LICENSE or
// <http://creativecommons.org/publicdomain/zero/1.0/> for full details.

package bulb

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"strings"
)

// Authenticate authenticates with the Tor instance using the "best" possible
// authentication method.  The password argument is entirely optional, and will
// only be used if the "SAFECOOKE" and "NULL" authentication methods are not
// available and "HASHEDPASSWORD" is.
func (c *Conn) Authenticate(password string) error {
	if c.isAuthenticated {
		return nil
	}

	// Determine the supported authentication methods, and the cookie path.
	pi, err := c.ProtocolInfo()
	if err != nil {
		return err
	}

	// "COOKIE" authentication exists, but anything modern supports
	// "SAFECOOKIE".
	const (
		cmdAuthenticate      = "AUTHENTICATE"
		authMethodNull       = "NULL"
		authMethodPassword   = "HASHEDPASSWORD"
		authMethodSafeCookie = "SAFECOOKIE"
	)
	if pi.AuthMethods[authMethodNull] {
		_, err = c.Request(cmdAuthenticate)
		c.isAuthenticated = err == nil
		return err
	} else if pi.AuthMethods[authMethodSafeCookie] {
		const (
			authCookieLength = 32
			authNonceLength  = 32
			authHashLength   = 32

			authServerHashKey = "Tor safe cookie authentication server-to-controller hash"
			authClientHashKey = "Tor safe cookie authentication controller-to-server hash"
		)

		if pi.CookieFile == "" {
			return newProtocolError("invalid (empty) COOKIEFILE")
		}
		cookie, err := ioutil.ReadFile(pi.CookieFile)
		if err != nil {
			return newProtocolError("failed to read COOKIEFILE: %v", err)
		} else if len(cookie) != authCookieLength {
			return newProtocolError("invalid cookie file length: %d", len(cookie))
		}

		// Send an AUTHCHALLENGE command, and parse the response.
		var clientNonce [authNonceLength]byte
		if _, err := rand.Read(clientNonce[:]); err != nil {
			return newProtocolError("failed to generate clientNonce: %v", err)
		}
		clientNonceStr := hex.EncodeToString(clientNonce[:])
		resp, err := c.Request("AUTHCHALLENGE %s %s", authMethodSafeCookie, clientNonceStr)
		if err != nil {
			return err
		}
		splitResp := strings.Split(resp.Reply, " ")
		if len(splitResp) != 3 {
			return newProtocolError("invalid AUTHCHALLENGE response")
		}
		serverHashStr := strings.TrimPrefix(splitResp[1], "SERVERHASH=")
		if serverHashStr == splitResp[1] {
			return newProtocolError("missing SERVERHASH")
		}
		serverHash, err := hex.DecodeString(serverHashStr)
		if err != nil {
			return newProtocolError("failed to decode ServerHash: %v", err)
		}
		if len(serverHash) != authHashLength {
			return newProtocolError("invalid ServerHash length: %d", len(serverHash))
		}
		serverNonceStr := strings.TrimPrefix(splitResp[2], "SERVERNONCE=")
		if serverNonceStr == splitResp[2] {
			return newProtocolError("missing SERVERNONCE")
		}
		serverNonce, err := hex.DecodeString(serverNonceStr)
		if err != nil {
			return newProtocolError("failed to decode ServerNonce: %v", err)
		}
		if len(serverNonce) != authNonceLength {
			return newProtocolError("invalid ServerNonce length: %d", len(serverNonce))
		}

		// Validate the ServerHash.
		m := hmac.New(sha256.New, []byte(authServerHashKey))
		m.Write(cookie)
		m.Write(clientNonce[:])
		m.Write(serverNonce)
		dervServerHash := m.Sum(nil)
		if !hmac.Equal(serverHash, dervServerHash) {
			return newProtocolError("invalid ServerHash: mismatch")
		}

		// Calculate the ClientHash, and issue the AUTHENTICATE.
		m = hmac.New(sha256.New, []byte(authClientHashKey))
		m.Write(cookie)
		m.Write(clientNonce[:])
		m.Write(serverNonce)
		clientHash := m.Sum(nil)
		clientHashStr := hex.EncodeToString(clientHash)

		_, err = c.Request("%s %s", cmdAuthenticate, clientHashStr)
		c.isAuthenticated = err == nil
		return err
	} else if pi.AuthMethods[authMethodPassword] {
		// Despite the name HASHEDPASSWORD, the raw password is actually sent.
		// According to the code, this can either be a QuotedString, or base16
		// encoded, so go with the later since it's easier to handle.
		if password == "" {
			return newProtocolError("password auth needs a password")
		}
		passwordStr := hex.EncodeToString([]byte(password))
		_, err = c.Request("%s %s", cmdAuthenticate, passwordStr)
		c.isAuthenticated = err == nil
		return err
	}
	return newProtocolError("no supported authentication methods")
}
