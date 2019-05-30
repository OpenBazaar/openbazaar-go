package repo

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
)

// NewPeerIDFromProtobuf translates a pb.ID protobuf into a PeerID
func NewPeerIDFromProtobuf(id *pb.ID) *PeerID {
	return &PeerID{
		hash:   id.PeerID,
		handle: id.Handle,
		keychain: &Pubkeys{
			identity: id.Pubkeys.Identity,
			bitcoin:  id.Pubkeys.Bitcoin,
		},
		bitcoinSignature: id.BitcoinSig,
	}
}

// Pubkeys holds bytes representing key material
type Pubkeys struct {
	bitcoin  []byte
	identity []byte
}

// PeerID represents a signed identity on OpenBazaar
type PeerID struct {
	hash             string
	handle           string
	keychain         *Pubkeys
	bitcoinSignature []byte
}

func (p *PeerID) Hash() string             { return p.hash }
func (p *PeerID) Handle() string           { return p.handle }
func (p *PeerID) BitcoinSignature() []byte { return p.bitcoinSignature }
func (p *PeerID) BitcoinKey() []byte       { return p.keychain.bitcoin }
func (p *PeerID) IdentityKey() []byte      { return p.keychain.identity }
