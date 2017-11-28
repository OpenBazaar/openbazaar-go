// cmd_onion.go - various onion service commands: ADD_ONION, DEL_ONION...
//
// To the extent possible under law, David Stainton and Ivan Markin waived
// all copyright and related or neighboring rights to this module of bulb,
// using the creative commons "cc0" public domain dedication. See LICENSE or
// <http://creativecommons.org/publicdomain/zero/1.0/> for full details.

package bulb

import (
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/yawning/bulb/utils/pkcs1"
)

// OnionInfo is the result of the AddOnion command.
type OnionInfo struct {
	OnionID    string
	PrivateKey crypto.PrivateKey

	RawResponse *Response
}

// OnionPrivateKey is a unknown Onion private key (crypto.PublicKey).
type OnionPrivateKey struct {
	KeyType string
	Key     string
}

// OnionPortSpec is a Onion VirtPort/Target pair.
type OnionPortSpec struct {
	VirtPort uint16
	Target   string
}

// NewOnionConfig is a configuration for NewOnion command.
type NewOnionConfig struct {
	PortSpecs    []OnionPortSpec
	PrivateKey   crypto.PrivateKey
	DiscardPK    bool
	Detach       bool
	BasicAuth    bool
	NonAnonymous bool
}

// NewOnion issues an ADD_ONION command using configuration config and
// returns the parsed response.
func (c *Conn) NewOnion(config *NewOnionConfig) (*OnionInfo, error) {
	const keyTypeRSA = "RSA1024"
	var err error

	var portStr string
	if config.PortSpecs == nil {
		return nil, newProtocolError("invalid port specification")
	}
	for _, v := range config.PortSpecs {
		portStr += fmt.Sprintf(" Port=%d", v.VirtPort)
		if v.Target != "" {
			portStr += "," + v.Target
		}
	}

	var hsKeyType, hsKeyStr string
	if config.PrivateKey != nil {
		switch t := config.PrivateKey.(type) {
		case *rsa.PrivateKey:
			rsaPK, _ := config.PrivateKey.(*rsa.PrivateKey)
			if rsaPK.N.BitLen() != 1024 {
				return nil, newProtocolError("invalid RSA key size")
			}
			pkDER, err := pkcs1.EncodePrivateKeyDER(rsaPK)
			if err != nil {
				return nil, newProtocolError("failed to serialize RSA key: %v", err)
			}
			hsKeyType = keyTypeRSA
			hsKeyStr = base64.StdEncoding.EncodeToString(pkDER)
		case *OnionPrivateKey:
			genericPK, _ := config.PrivateKey.(*OnionPrivateKey)
			hsKeyType = genericPK.KeyType
			hsKeyStr = genericPK.Key
		default:
			return nil, newProtocolError("unsupported private key type: %v", t)
		}
	} else {
		hsKeyStr = "BEST"
		hsKeyType = "NEW"
	}

	var flags []string
	var flagsStr string

	if config.DiscardPK {
		flags = append(flags, "DiscardPK")
	}
	if config.Detach {
		flags = append(flags, "Detach")
	}
	if config.BasicAuth {
		flags = append(flags, "BasicAuth")
	}
	if config.NonAnonymous {
		flags = append(flags, "NonAnonymous")
	}
	if flags != nil {
		flagsStr = " Flags="
		flagsStr += strings.Join(flags, ",")
	}
	request := fmt.Sprintf("ADD_ONION %s:%s%s%s", hsKeyType, hsKeyStr, portStr, flagsStr)
	resp, err := c.Request(request)
	if err != nil {
		return nil, err
	}

	// Parse out the response.
	var serviceID string
	var hsPrivateKey crypto.PrivateKey
	for _, l := range resp.Data {
		const (
			serviceIDPrefix  = "ServiceID="
			privateKeyPrefix = "PrivateKey="
		)

		if strings.HasPrefix(l, serviceIDPrefix) {
			serviceID = strings.TrimPrefix(l, serviceIDPrefix)
		} else if strings.HasPrefix(l, privateKeyPrefix) {
			if config.DiscardPK || hsKeyStr != "" {
				return nil, newProtocolError("received an unexpected private key")
			}
			hsKeyStr = strings.TrimPrefix(l, privateKeyPrefix)
			splitKey := strings.SplitN(hsKeyStr, ":", 2)
			if len(splitKey) != 2 {
				return nil, newProtocolError("failed to parse private key type")
			}

			switch splitKey[0] {
			case keyTypeRSA:
				keyBlob, err := base64.StdEncoding.DecodeString(splitKey[1])
				if err != nil {
					return nil, newProtocolError("failed to base64 decode RSA key: %v", err)
				}
				hsPrivateKey, _, err = pkcs1.DecodePrivateKeyDER(keyBlob)
				if err != nil {
					return nil, newProtocolError("failed to deserialize RSA key: %v", err)
				}
			default:
				hsPrivateKey := new(OnionPrivateKey)
				hsPrivateKey.KeyType = splitKey[0]
				hsPrivateKey.Key = splitKey[1]
			}
		}
	}
	if serviceID == "" {
		// This should *NEVER* happen, since the command succeded, and the spec
		// guarantees that this will always be present.
		return nil, newProtocolError("failed to determine service ID")
	}

	oi := new(OnionInfo)
	oi.RawResponse = resp
	oi.OnionID = serviceID
	oi.PrivateKey = hsPrivateKey

	return oi, nil
}

// [DEPRECATED] AddOnion issues an ADD_ONION command and
// returns the parsed response.
func (c *Conn) AddOnion(ports []OnionPortSpec, key crypto.PrivateKey, oneshot bool) (*OnionInfo, error) {
	cfg := &NewOnionConfig{}
	cfg.PortSpecs = ports
	if key != nil {
		cfg.PrivateKey = key
	}
	cfg.DiscardPK = oneshot
	return c.NewOnion(cfg)
}

// DeleteOnion issues a DEL_ONION command and returns the parsed response.
func (c *Conn) DeleteOnion(serviceID string) error {
	_, err := c.Request("DEL_ONION %s", serviceID)
	return err
}
