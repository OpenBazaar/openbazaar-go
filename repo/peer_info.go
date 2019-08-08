package repo

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	crypto "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
)

var (
	ErrInvalidInlinePeerID = errors.New("inline hash does not match produced hash")
	ErrPeerInfoIsNil       = errors.New("peer info is nil")
)

func init() {
	peer.AdvancedEnableInlining = false
}

// NewPeerInfoFromProtobuf translates a pb.ID protobuf into a PeerInfo
func NewPeerInfoFromProtobuf(id *pb.ID) (*PeerInfo, error) {
	return &PeerInfo{
		protobufPeerID: id.PeerID,
		handle:         id.Handle,
		keychain: &PeerKeychain{
			identity: id.Pubkeys.Identity,
			bitcoin:  id.Pubkeys.Bitcoin,
		},
		bitcoinSignature: id.BitcoinSig,
	}, nil
}

// NewPeerInfoFromIdentityKey returns a PeerInfo object based on the identity key
func NewPeerInfoFromIdentityKey(k []byte) *PeerInfo {
	return &PeerInfo{
		keychain: &PeerKeychain{identity: k},
	}
}

// PeerKeychain holds bytes representing key material suitable for extracting with libp2p-crypto
type PeerKeychain struct {
	bitcoin  []byte
	identity []byte
}

// PeerInfo represents a signed identity on OpenBazaar
type PeerInfo struct {
	protobufPeerID string
	peerHashMemo   string

	handle           string
	keychain         *PeerKeychain
	bitcoinSignature []byte
}

func (p *PeerInfo) String() string {
	return fmt.Sprintf("&PeerInfo{protobufPeerID:%s handle:%s bitcoinSignature:%v keychain: &PeerKeychain{bitcoin:%v identity:%v}}", p.protobufPeerID, p.handle, p.bitcoinSignature, p.keychain.bitcoin, p.keychain.identity)
}

func (p *PeerInfo) Handle() string { return p.handle }
func (p *PeerInfo) BitcoinSignature() []byte {
	if p == nil {
		return nil
	}
	var sig = make([]byte, len(p.bitcoinSignature))
	copy(sig, p.bitcoinSignature)
	return sig
}
func (p *PeerInfo) BitcoinKey() []byte {
	var key = make([]byte, len(p.keychain.bitcoin))
	copy(key, p.keychain.bitcoin)
	return key
}
func (p *PeerInfo) IdentityKeyBytes() []byte {
	var key = make([]byte, len(p.keychain.identity))
	copy(key, p.keychain.identity)
	return key
}
func (p *PeerInfo) IdentityKey() (ipfs.PubKey, error) {
	key, err := crypto.UnmarshalPublicKey(p.IdentityKeyBytes())
	if err != nil {
		return nil, fmt.Errorf("unmarshaling identity key bytes: %s", err)
	}
	return key.(ipfs.PubKey), nil
}

func (p *PeerInfo) Equal(other *PeerInfo) bool {
	if p == nil || other == nil {
		return false
	}
	if !bytes.Equal(p.IdentityKeyBytes(), other.IdentityKeyBytes()) {
		return false
	}
	if !bytes.Equal(p.BitcoinKey(), other.BitcoinKey()) {
		return false
	}
	if !bytes.Equal(p.BitcoinSignature(), other.BitcoinSignature()) {
		return false
	}
	if p.handle != other.handle {
		return false
	}
	tHash, err := p.Hash()
	if err != nil {
		return false
	}
	oHash, err := other.Hash()
	if err != nil {
		return false
	}
	if tHash != oHash {
		return false
	}
	return true
}

func (p *PeerInfo) Valid() (result bool, errs []error) {
	if p == nil {
		return false, []error{ErrPeerInfoIsNil}
	}
	result = true
	errs = make([]error, 0)
	if p.protobufPeerID != "" {
		hash, err := p.Hash()
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to produce peer hash: %s", err))
			result = false
		} else {
			if hash != p.protobufPeerID {
				errs = append(errs, ErrInvalidInlinePeerID)
				result = false
			}
		}
	}
	// TODO: validate BitcoinSignature comes from bitcoin identity
	return
}

// Hash returns the public hash based on the PeerKeychain.Identity key material
func (p *PeerInfo) Hash() (string, error) {
	if p == nil {
		return "", ErrPeerInfoIsNil
	}
	if p.peerHashMemo != "" {
		return p.peerHashMemo, nil
	}

	key, err := p.IdentityKey()
	if err != nil {
		return "", err
	}
	id, err := peer.IDFromPublicKey(key)
	if err != nil {
		return "", err
	}
	p.peerHashMemo = id.Pretty()
	return p.peerHashMemo, nil
}

func (p *PeerInfo) Protobuf() *pb.ID {
	peerHash, err := p.Hash()
	if err != nil && p.protobufPeerID != "" {
		peerHash = p.protobufPeerID
	}
	return &pb.ID{
		PeerID: peerHash,
		Handle: p.handle,
		Pubkeys: &pb.ID_Pubkeys{
			Bitcoin:  p.BitcoinKey(),
			Identity: p.IdentityKeyBytes(),
		},
	}
}