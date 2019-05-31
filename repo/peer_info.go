package repo

import (
	"errors"
	"fmt"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	crypto "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
)

var ErrInvalidInlinePeerID = errors.New("inline hash does not match produced hash")

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
	protobufPeerID   string
	handle           string
	keychain         *PeerKeychain
	bitcoinSignature []byte
}

func (p *PeerInfo) Handle() string           { return p.handle }
func (p *PeerInfo) BitcoinSignature() []byte { return p.bitcoinSignature }
func (p *PeerInfo) BitcoinKey() []byte       { return p.keychain.bitcoin }
func (p *PeerInfo) IdentityKeyBytes() []byte { return p.keychain.identity }
func (p *PeerInfo) IdentityKey() (ipfs.PubKey, error) {
	key, err := crypto.UnmarshalPublicKey(p.IdentityKeyBytes())
	if err != nil {
		return nil, fmt.Errorf("reading: %s", err)
	}
	return key.(ipfs.PubKey), nil
}

func (p *PeerInfo) Valid() (result bool, errs []error) {
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
	key, err := p.IdentityKey()
	if err != nil {
		return "", err
	}
	id, err := peer.IDFromPublicKey(key)
	if err != nil {
		return "", err
	}
	return id.Pretty(), nil
}
