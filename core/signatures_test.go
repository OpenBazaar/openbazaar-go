package core_test

import (
	"encoding/hex"
	crypto "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	"strings"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/ipfs/go-ipfs/core/mock"
)

func TestSigningingPayload(t *testing.T) {
	var node, err = coremock.NewMockNode()
	if err != nil {
		t.Fatal(err)
	}

	var (
		errInvalidKey = "invalid key"
		examples      = []struct {
			payload     []byte
			key         crypto.PrivKey
			expectedErr *string
		}{
			{ // empty byteslice is valid
				payload:     []byte(""),
				key:         node.PrivateKey,
				expectedErr: nil,
			},
			{ // filled byteslice is valid
				payload:     []byte("qwertyuiopasdfghjklzxcvbnm"),
				key:         node.PrivateKey,
				expectedErr: nil,
			},
			{ // missing key throws error
				payload:     []byte("qwertyuiopasdfghjklzxcvbnm"),
				key:         nil,
				expectedErr: &errInvalidKey,
			},
		}
	)

	for _, e := range examples {
		sig, _, err := core.SignPayload(e.payload, e.key)
		if err != nil {
			if e.expectedErr != nil {
				if !strings.Contains(err.Error(), *e.expectedErr) {
					t.Errorf("expected error to contain (%s), but did not: %s", *e.expectedErr, err.Error())
					t.Logf("\texample payload: (%s) key: (%t)", string(e.payload), e.key != nil)
					continue
				}
			} else {
				t.Errorf("unexpected error: %s", err.Error())
				t.Logf("\texample payload: (%s) key: (%t)", string(e.payload), e.key != nil)
			}
			continue
		} else {
			if e.expectedErr != nil {
				t.Errorf("expected error (%s), but produced none", *e.expectedErr)
				t.Logf("\texample payload: (%s) key: (%t)", string(e.payload), e.key != nil)
				continue
			}
		}

		expectedSig, err := e.key.Sign(e.payload)
		if err != nil {
			t.Errorf("producing expected sig: %s", err.Error())
			continue
		}
		hexSig := hex.EncodeToString(sig)
		if hex.EncodeToString(expectedSig) != hexSig {
			t.Errorf("unexpected signature")
			t.Logf("\texample payload (%s)", e.payload)
			t.Logf("\texpected (%s)", hex.EncodeToString(sig))
			t.Logf("\tactual (%s)", hexSig)
		}
	}
}

func TestVerifyingPayload(t *testing.T) {
	var node, err = coremock.NewMockNode()
	if err != nil {
		t.Fatal(err)
	}
	var pubKeyBytes, pErr = node.PrivateKey.GetPublic().Bytes()
	if pErr != nil {
		t.Fatal(pErr)
	}

	var (
		errInvalidKey   = "invalid key"
		errVerification = "verification error"
		validSig        = func(payload []byte) []byte {
			sig, err := node.PrivateKey.Sign(payload)
			if err != nil {
				t.Fatal(err)
			}
			return sig
		}
		examples = []struct {
			payload     []byte
			sigFunc     func([]byte) []byte
			pubkey      []byte
			expectedErr *string
		}{
			{ // empty payload is valid
				payload:     []byte(""),
				sigFunc:     validSig,
				pubkey:      pubKeyBytes,
				expectedErr: nil,
			},
			{ // empty sig is an error
				payload:     []byte("asdfghjkl"),
				sigFunc:     func(_ []byte) []byte { return []byte("") },
				pubkey:      pubKeyBytes,
				expectedErr: &errVerification,
			},
			{ // empty pubkey is an error
				payload:     []byte("asdfghjkl"),
				sigFunc:     validSig,
				pubkey:      []byte(""),
				expectedErr: &errInvalidKey,
			},
		}
	)

	for _, e := range examples {
		_, err := core.VerifyPayload(e.payload, e.sigFunc(e.payload), e.pubkey)
		if err != nil {
			if e.expectedErr != nil {
				if !strings.Contains(err.Error(), *e.expectedErr) {
					t.Errorf("expected error to contain (%s), but did not: %s", *e.expectedErr, err.Error())
					t.Logf("\texample payload: (%s) key: (%t)", string(e.payload), len(e.pubkey) > 0)
					continue
				}
			} else {
				t.Errorf("unexpected error: %s", err.Error())
				t.Logf("\texample payload: (%s) key: (%t)", string(e.payload), len(e.pubkey) > 0)
			}
			continue
		} else {
			if e.expectedErr != nil {
				t.Errorf("expected error (%s), but produced none", *e.expectedErr)
				t.Logf("\texample payload: (%s) key: (%t)", string(e.payload), len(e.pubkey) > 0)
				continue
			}
		}

	}
}

func TestSignAndVerifyAreReciprocalFunctions(t *testing.T) {
	var (
		payload   = []byte("testing")
		node, err = coremock.NewMockNode()
	)
	if err != nil {
		t.Fatal(err)
	}

	sig, pubkey, err := core.SignPayload(payload, node.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := core.VerifyPayload(payload, sig, pubkey); err != nil {
		t.Errorf("expected payload to verify: %s", err.Error())
	}
}
