package repo

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
)

// SignedListing represents a finalized listing which has been
// signed by the vendor
type SignedListing struct {
	Hash         string      `json:"hash"`
	Signature    []byte      `json:"signature"`
	RListing     Listing     `json:"listing"`
	ProtoListing *pb.Listing `json:"-"`

	ProtoSignedListing *pb.SignedListing `json:"-"`

	proto.Message
}

func (l *SignedListing) Reset()         { *l = SignedListing{} }
func (l *SignedListing) String() string { return proto.CompactTextString(l) }
func (*SignedListing) ProtoMessage()    {}

// GetVersion returns the schema version
func (l *SignedListing) GetVersion() uint { return l.RListing.Metadata.Version }

// GetTitle returns the title
func (l *SignedListing) GetTitle() (string, error) { return l.RListing.GetTitle() }

// GetSlug returns the slug
func (l *SignedListing) GetSlug() (string, error) { return l.RListing.GetSlug() }

// GetPrice returns the price
func (l *SignedListing) GetPrice() (*CurrencyValue, error) { return l.RListing.GetPrice() }

// GetAcceptedCurrencies returns the list of currencies which the listing
// may be purchased with
func (l *SignedListing) GetAcceptedCurrencies() ([]string, error) {

	return l.RListing.GetAcceptedCurrencies()

}

// GetCryptoDivisibility returns the divisibility of a cryptocurrency's
// listing sold inventory
func (l *SignedListing) GetCryptoDivisibility() uint32 { return l.RListing.GetCryptoDivisibility() }

// GetCryptoCurrencyCode returns the currency code of the sold
// cryptocurrency listing
func (l *SignedListing) GetCryptoCurrencyCode() string { return l.RListing.GetCryptoCurrencyCode() }
