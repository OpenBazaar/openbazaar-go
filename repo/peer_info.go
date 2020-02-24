package repo

import (
	"bytes"
	"errors"
	"fmt"

	crypto "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/btcsuite/btcd/btcec"
)

var (
	ErrInvalidInlinePeerID = errors.New("inline hash does not match produced hash")
	ErrPeerInfoIsNil       = errors.New("peer info is nil")
)

func init() {
	peer.AdvancedEnableInlining = false
}

// NewPeerInfoFromProtobuf translates a pb.ID protobuf into a PeerInfo
func NewPeerInfoFromProtobuf(id *pb.ID) *PeerInfo {
	return &PeerInfo{
		protobufPeerID: id.PeerID,
		handle:         id.Handle,
		keychain: &PeerKeychain{
			identity: id.Pubkeys.Identity,
			bitcoin:  id.Pubkeys.Bitcoin,
		},
		bitcoinSignature: id.BitcoinSig,
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

	handle           string
	keychain         *PeerKeychain
	bitcoinSignature []byte
}

func (p *PeerInfo) String() string {
	return fmt.Sprintf("&PeerInfo{protobufPeerID:%s handle:%s bitcoinSignature:%v keychain: &PeerKeychain{bitcoin:%v identity:%v}}",
		p.protobufPeerID,
		p.handle,
		p.bitcoinSignature,
		p.keychain.bitcoin,
		p.keychain.identity,
	)
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

// Hash returns the public hash based on the PeerKeychain.Identity key material
func (p *PeerInfo) Hash() (string, error) {
	if p == nil {
		return "", ErrPeerInfoIsNil
	}
	if p.protobufPeerID == "" {
		return "", ErrInvalidInlinePeerID
	}
	return p.protobufPeerID, nil
}

func (p *PeerInfo) GeneratePeerIDFromIdentityKey() (string, error) {
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

// Valid ensures the PeerInfo is valid as derived by the provided protobuf
func (p *PeerInfo) Valid() error {
	if p == nil {
		return ErrPeerInfoIsNil
	}
	if err := p.VerifyIdentity(); err != nil {
		return err
	}
	if err := p.VerifyBitcoinSignature(); err != nil {
		return err
	}
	// TODO: validate BitcoinSignature comes from bitcoin identity
	return nil
}

// VerifyIdentity checks that the peer id, identity key both agree
func (p *PeerInfo) VerifyIdentity() error {
	hash, err := p.Hash()
	if err != nil {
		return fmt.Errorf("unable to produce peer hash: %s", err.Error())
	}

	peerID, err := peer.IDB58Decode(hash)
	if err != nil {
		return fmt.Errorf("decoding peer hash: %s", err.Error())
	}
	pub, err := crypto.UnmarshalPublicKey(p.IdentityKeyBytes())
	if err != nil {
		return fmt.Errorf("parsing identity key: %s", err.Error())
	}
	if !peerID.MatchesPublicKey(pub) {
		return ErrInvalidInlinePeerID
	}
	return nil
}

// VerifyBitcoinSignature checks that the bitcoin key and the peer id both agree
func (p *PeerInfo) VerifyBitcoinSignature() error {
	bitcoinPubkey, err := btcec.ParsePubKey(p.BitcoinKey(), btcec.S256())
	if err != nil {
		return fmt.Errorf("parse bitcoin pubkey: %s", err.Error())
	}
	bitcoinSig, err := btcec.ParseSignature(p.BitcoinSignature(), btcec.S256())
	if err != nil {
		return fmt.Errorf("parse bitcoin signature: %s", err.Error())
	}
	pid, err := p.Hash()
	if err != nil {
		return fmt.Errorf("get peer id: %s", err.Error())
	}
	if !bitcoinSig.Verify([]byte(pid), bitcoinPubkey) {
		return errors.New("bitcoin signature on peer id did not verify successfully")
	}
	return nil
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
		BitcoinSig: p.bitcoinSignature,
	}
}
