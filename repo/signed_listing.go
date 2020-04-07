package repo

import (
	"errors"
	"fmt"

	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
)

// UnmarshalJSONSignedListing extracts a SignedListing from marshaled JSON
func UnmarshalJSONSignedListing(data []byte) (SignedListing, error) {
	var (
		sl  = new(pb.SignedListing)
		err = jsonpb.UnmarshalString(string(data), sl)
	)
	if err != nil {
		return SignedListing{}, err
	}
	return SignedListing{
		signedListingProto: sl,
	}, nil
}

func NewSignedListingFromProtobuf(sl *pb.SignedListing) SignedListing {
	return SignedListing{
		signedListingProto: sl,
	}
}

// SignedListing represents a finalized listing which has been
// signed by the vendor
type SignedListing struct {
	signedListingProto *pb.SignedListing

	proto.Message
}

func (l *SignedListing) Reset()         { *l = SignedListing{} }
func (l *SignedListing) String() string { return proto.CompactTextString(l) }
func (*SignedListing) ProtoMessage()    {}

// Normalize is a helper method which will mutate the listing protobuf
// in-place but maintain the original signature for external verification
// purposes.
func (l *SignedListing) Normalize() error {
	nl, err := l.GetListing().Normalize()
	if err != nil {
		return err
	}

	l.signedListingProto.Listing = nl.listingProto
	return nil
}

// GetListing returns the underlying repo.Listing object
func (l SignedListing) GetListing() *Listing {
	return &Listing{
		listingProto: l.signedListingProto.Listing,
	}
}

// GetVersion returns the schema version
func (l SignedListing) GetVersion() uint32 { return l.GetListing().GetVersion() }

// GetVendorID returns the PeerInfo for the listing
func (l SignedListing) GetVendorID() *PeerInfo { return l.GetListing().GetVendorID() }

// GetTitle returns the title
func (l SignedListing) GetTitle() string { return l.GetListing().GetTitle() }

// GetSlug returns the slug
func (l SignedListing) GetSlug() string { return l.GetListing().GetSlug() }

// GetPrice returns the price
func (l SignedListing) GetPrice() (*CurrencyValue, error) { return l.GetListing().GetPrice() }

// GetAcceptedCurrencies returns the list of currencies which the listing
// may be purchased with
func (l SignedListing) GetAcceptedCurrencies() []string {
	return l.GetListing().GetAcceptedCurrencies()
}

// GetCryptoDivisibility returns the divisibility of a cryptocurrency's
// listing sold inventory
func (l SignedListing) GetCryptoDivisibility() uint32 { return l.GetListing().GetCryptoDivisibility() }

// GetCryptoCurrencyCode returns the currency code of the sold
// cryptocurrency listing
func (l SignedListing) GetCryptoCurrencyCode() string { return l.GetListing().GetCryptoCurrencyCode() }

// GetSignature returns the signature on the listing
func (l SignedListing) GetSignature() []byte {
	return l.signedListingProto.GetSignature()
}

// GetListingSigProtobuf returns the protobuf signature suitable for attaching
// to a pb.RicardianContract
func (l SignedListing) GetListingSigProtobuf() *pb.Signature {
	return &pb.Signature{
		Section:        pb.Signature_LISTING,
		SignatureBytes: l.GetSignature(),
	}
}

func (l SignedListing) MarshalJSON() ([]byte, error) {
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "    ",
		OrigName:     false,
	}
	lb, err := m.MarshalToString(l.signedListingProto)
	if err != nil {
		return nil, err
	}
	return []byte(lb), nil
}

// ValidateListing ensures all listing state is valid
func (l SignedListing) ValidateListing(isTestnet bool) error {
	return l.GetListing().ValidateListing(isTestnet)
}

// VerifySignature checks the listings signature was produced by the vendor's
// Identity key and that the key was derived from the vendor's peerID
func (l SignedListing) VerifySignature() error {
	ser, err := l.GetListing().MarshalProtobuf()
	if err != nil {
		return fmt.Errorf("marshaling listing: %s", err.Error())
	}

	pubkey, err := l.GetListing().GetVendorID().IdentityKey()
	if err != nil {
		return fmt.Errorf("getting identity pubkey: %s", err.Error())
	}
	valid, err := pubkey.Verify(ser, l.GetSignature())
	if err != nil {
		return fmt.Errorf("verifying signature: %s", err.Error())
	}
	if !valid {
		return errors.New("identity signature on contract failed to verify")
	}

	peerHash, err := l.GetListing().GetVendorID().Hash()
	if err != nil {
		return fmt.Errorf("get peer id: %s", err.Error())
	}
	pid, err := peer.IDB58Decode(peerHash)
	if err != nil {
		return fmt.Errorf("decoding peer id: %s", err.Error())
	}
	if !pid.MatchesPublicKey(pubkey) {
		return errors.New("vendor's identity key does not match peer id")
	}
	return nil
}
