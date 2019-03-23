package core

import (
	"errors"
	"fmt"

	crypto "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/btcsuite/btcd/btcec"
	"github.com/golang/protobuf/proto"
)

var ErrInvalidKey = errors.New("invalid key")

type noSigError struct{}
type invalidSigError struct{}
type matchKeyError struct{}

func (noSigError) Error() string {
	return "Contract does not contain a signature for the given section"
}

func (invalidSigError) Error() string {
	return "Invalid signature"
}

func (matchKeyError) Error() string {
	return "Signature does not match public key"
}

// verifyMessageSignature accepts message, public key butes, list of signatures,
// signature section to be looked up in this list, and GUID string. Returns an
// error, with special cases:
//     - noSigError (signature not present in the list);
//     - invalidSigError (signature is invalid);
//     - matchKeyError (GUID does not match public key).
// Example usage can be seen in verifySignaturesOnOrder() in 'core/order.go'.
func verifyMessageSignature(msg proto.Message, pk []byte, sigs []*pb.Signature, sigType pb.Signature_Section, peerID string) error {
	sig, err := selectSignature(sigs, sigType)
	if err != nil {
		return err
	}
	return verifySignature(msg, pk, sig.SignatureBytes, peerID)
}

func verifySignature(msg proto.Message, pk []byte, signature []byte, peerID string) error {
	ser, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	pubkey, err := crypto.UnmarshalPublicKey(pk)
	if err != nil {
		return err
	}
	valid, err := pubkey.Verify(ser, signature)
	if err != nil {
		return err
	}
	if !valid {
		return invalidSigError{}
	}
	pid, err := peer.IDB58Decode(peerID)
	if err != nil {
		return err
	}
	if !pid.MatchesPublicKey(pubkey) {
		return matchKeyError{}
	}
	return nil
}

func verifyBitcoinSignature(pubkeyBytes, sigBytes []byte, guid string) error {
	bitcoinPubkey, err := btcec.ParsePubKey(pubkeyBytes, btcec.S256())
	if err != nil {
		return err
	}
	bitcoinSig, err := btcec.ParseSignature(sigBytes, btcec.S256())
	if err != nil {
		return err
	}
	if !bitcoinSig.Verify([]byte(guid), bitcoinPubkey) {
		return invalidSigError{}
	}
	return nil
}

func selectSignature(signatures []*pb.Signature, sigType pb.Signature_Section) (*pb.Signature, error) {
	var sig *pb.Signature
	var err error = noSigError{}
	for _, s := range signatures {
		if s.Section == sigType {
			sig, err = s, nil
		}
	}
	return sig, err
}

// SignPayload produces a signature for the given private key and payload
// and returns it and the public key or an error
func SignPayload(payload []byte, privKey crypto.PrivKey) ([]byte, []byte, error) {
	if privKey == nil {
		return nil, nil, ErrInvalidKey
	}
	var (
		sig, sErr    = privKey.Sign(payload)
		pubkey, pErr = privKey.GetPublic().Bytes()
	)
	if sErr != nil {
		return nil, nil, fmt.Errorf("signing payload: %s", sErr.Error())
	}
	if pErr != nil {
		return nil, nil, fmt.Errorf("getting pub key: %s", pErr.Error())
	}
	return sig, pubkey, nil
}

// VerifyPayload proves the payload and signature are authentic
// for the provided public key and returns the peer ID for that
// pubkey with no error on success
func VerifyPayload(payload, sig, pubKey []byte) (string, error) {
	if len(pubKey) == 0 {
		return "", ErrInvalidKey
	}
	pk, err := crypto.UnmarshalPublicKey(pubKey)
	if err != nil {
		return "", err
	}

	_, err = pk.Verify(payload, sig)
	if err != nil {
		return "", err
	}

	peerID, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return "", err
	}

	return peerID.Pretty(), nil
}
