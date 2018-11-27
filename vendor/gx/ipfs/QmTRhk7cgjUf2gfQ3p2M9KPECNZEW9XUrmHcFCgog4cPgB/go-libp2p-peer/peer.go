// Package peer implements an object used to represent peers in the ipfs network.
package peer

import (
	"encoding/hex"
	"errors"
	"fmt"

	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	ic "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	b58 "gx/ipfs/QmWFAMPqsEyUX7gDUsRVmMWz59FxSpJ1b2v6bJ1yYzo7jY/go-base58-fast/base58"
	mc "gx/ipfs/QmNhVCV7kgAqW6oh6n8m9myxT2ksGPhVZnHkzkBvR5qg2d/go-multicodec-packed"

)

// MaxInlineKeyLength is the maximum length a key can be for it to be inlined in
// the peer ID.
//
// * When `len(pubKey.Bytes()) <= MaxInlineKeyLength`, the peer ID is the
//   identity multihash hash of the public key.
// * When `len(pubKey.Bytes()) > MaxInlineKeyLength`, the peer ID is the
//   sha2-256 multihash of the public key.
const MaxInlineKeyLength = 42

var (
	// ErrEmptyPeerID is an error for empty peer ID.
	ErrEmptyPeerID = errors.New("empty peer ID")
	// ErrNoPublickKey is an error for peer IDs that don't embed public keys
	ErrNoPublicKey = errors.New("public key is not embedded in peer ID")
)

// ID is a libp2p peer identity.
type ID string

// Pretty returns a b58-encoded string of the ID
func (id ID) Pretty() string {
	return IDB58Encode(id)
}

// Loggable returns a pretty peerID string in loggable JSON format
func (id ID) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"peerID": id.Pretty(),
	}
}

// String prints out the peer.
//
// TODO(brian): ensure correctness at ID generation and
// enforce this by only exposing functions that generate
// IDs safely. Then any peer.ID type found in the
// codebase is known to be correct.
func (id ID) String() string {
	pid := id.Pretty()
	if len(pid) <= 10 {
		return fmt.Sprintf("<peer.ID %s>", pid)
	}
	return fmt.Sprintf("<peer.ID %s*%s>", pid[:2], pid[len(pid)-6:])
}

// MatchesPrivateKey tests whether this ID was derived from sk
func (id ID) MatchesPrivateKey(sk ic.PrivKey) bool {
	return id.MatchesPublicKey(sk.GetPublic())
}

// MatchesPublicKey tests whether this ID was derived from pk
func (id ID) MatchesPublicKey(pk ic.PubKey) bool {
	oid, err := IDFromPublicKey(pk)
	if err != nil {
		return false
	}
	return oid == id
}

var MultihashDecodeErr = errors.New("unable to decode multihash")
var MultihashCodecErr = errors.New("unexpected multihash codec")
var MultihashLengthErr = errors.New("unexpected multihash length")
var CodePrefixErr = errors.New("unexpected code prefix")

func (id ID) ExtractEd25519PublicKey() (ic.PubKey, error) {
	// ed25519 pubkey identity format
	// <identity mc><length (2 + 32 = 34)><ed25519-pub mc><ed25519 pubkey>
	// <0x00       ><0x22                ><0xed01        ><ed25519 pubkey>

	var nilPubKey ic.PubKey

	// Decode multihash
	decoded, err := mh.Decode([]byte(id))
	if err != nil {
		return nilPubKey, MultihashDecodeErr
	}

	// Check ID multihash codec
	if decoded.Code != mh.ID {
		return nilPubKey, MultihashCodecErr
	}

	// Check multihash length
	if decoded.Length != 2+32 {
		return nilPubKey, MultihashLengthErr
	}

	// Split prefix
	code, pubKeyBytes := mc.SplitPrefix(decoded.Digest)

	// Check ed25519 code
	if code != mc.Ed25519Pub {
		return nilPubKey, CodePrefixErr
	}

	// Unmarshall public key
	pubKey, err := ic.UnmarshalEd25519PublicKey(pubKeyBytes)
	if err != nil {
		// Should never occur because of the check decoded.Length != 2+32
		return nilPubKey, fmt.Errorf("Unexpected error unmarshalling Ed25519 public key")
	}

	return pubKey, nil
}

// ExtractPublicKey attempts to extract the public key from an ID
//
// This method returns ErrNoPublicKey if the peer ID looks valid but it can't extract
// the public key.
func (id ID) ExtractPublicKey() (ic.PubKey, error) {
	decoded, err := mh.Decode([]byte(id))
	if err != nil {
		return nil, err
	}
	if decoded.Code != mh.ID {
		return nil, ErrNoPublicKey
	}
	pk, err := ic.UnmarshalPublicKey(decoded.Digest)
	if err != nil {
		return nil, err
	}
	return pk, nil
}

// Validate check if ID is empty or not
func (id ID) Validate() error {
	if id == ID("") {
		return ErrEmptyPeerID
	}

	return nil
}

// IDFromString cast a string to ID type, and validate
// the id to make sure it is a multihash.
func IDFromString(s string) (ID, error) {
	if _, err := mh.Cast([]byte(s)); err != nil {
		return ID(""), err
	}
	return ID(s), nil
}

// IDFromBytes cast a string to ID type, and validate
// the id to make sure it is a multihash.
func IDFromBytes(b []byte) (ID, error) {
	if _, err := mh.Cast(b); err != nil {
		return ID(""), err
	}
	return ID(b), nil
}

// IDB58Decode returns a b58-decoded Peer
func IDB58Decode(s string) (ID, error) {
	m, err := mh.FromB58String(s)
	if err != nil {
		return "", err
	}
	return ID(m), err
}

// IDB58Encode returns b58-encoded string
func IDB58Encode(id ID) string {
	return b58.Encode([]byte(id))
}

// IDHexDecode returns a hex-decoded Peer
func IDHexDecode(s string) (ID, error) {
	m, err := mh.FromHexString(s)
	if err != nil {
		return "", err
	}
	return ID(m), err
}

// IDHexEncode returns hex-encoded string
func IDHexEncode(id ID) string {
	return hex.EncodeToString([]byte(id))
}

func FlexPubKey(pk ic.PubKey) (ID, error) {
	b, err := pk.Bytes()
	if err != nil {
		return "", err
	}
	var alg uint64 = mh.SHA2_256
	if len(b) <= MaxInlineKeyLength {
		alg = mh.ID
	}
	hash, _ := mh.Sum(b, alg, -1)
	return ID(hash), nil
}

// IDFromPublicKey returns the Peer ID corresponding to pk
func IDFromPublicKey(pk ic.PubKey) (ID, error) {
	b, err := pk.Bytes()
	if err != nil {
		return "", err
	}
	var alg uint64 = mh.SHA2_256
	//if len(b) <= MaxInlineKeyLength {
	//	alg = mh.ID
	//}
	hash, _ := mh.Sum(b, alg, -1)
	return ID(hash), nil
}

// IDFromPrivateKey returns the Peer ID corresponding to sk
func IDFromPrivateKey(sk ic.PrivKey) (ID, error) {
	return IDFromPublicKey(sk.GetPublic())
}

// IDSlice for sorting peers
type IDSlice []ID

func (es IDSlice) Len() int           { return len(es) }
func (es IDSlice) Swap(i, j int)      { es[i], es[j] = es[j], es[i] }
func (es IDSlice) Less(i, j int) bool { return string(es[i]) < string(es[j]) }
