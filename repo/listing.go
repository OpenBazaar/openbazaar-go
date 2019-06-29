package repo

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
)

// NewListingFromProtobuf - return Listing from pb.Listing
func NewListingFromProtobuf(l *pb.Listing) (*Listing, error) {
	//var vendorInfo, err = NewPeerInfoFromProtobuf(l.VendorID)
	//if err != nil {
	//	return nil, fmt.Errorf("new peer info: %s", err)
	//}
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "    ",
		OrigName:     false,
	}

	out, err := m.MarshalToString(l)
	if err != nil {
		return nil, err
	}
	return &Listing{
		Slug:               l.Slug,
		TermsAndConditions: l.TermsAndConditions,
		RefundPolicy:       l.RefundPolicy,
		//Vendor:             vendorInfo,
		ListingBytes:   []byte(out),
		ListingVersion: l.Metadata.Version,
	}, nil
}

// IndividualListingContainer -
type IndividualListingContainer struct {
	Listing `json:"listing"`
}

// Listing represents a trade offer which can be accepted by another
// party on the OpenBazaar network
type Listing struct {
	Slug               string //`json:"slug"`
	TermsAndConditions string //`json:"termsAndConditions"`
	RefundPolicy       string //`json:"refundPolicy"`

	//Vendor   *PeerInfo       //`json:"vendorID"`
	Metadata ListingMetadata //`json:"metadata"`
	//Hash               string    `json:"hash"`
	//Title              string    `json:"title"`
	//Categories         []string  `json:"categories"`
	//NSFW               bool      `json:"nsfw"`
	//ContractType       string    `json:"contractType"`
	//Description        string    `json:"description"`
	//Thumbnail          thumbnail `json:"thumbnail"`
	//Price              price     `json:"price"`
	//ShipsTo            []string  `json:"shipsTo"`
	//FreeShipping       []string  `json:"freeShipping"`
	//Language           string    `json:"language"`
	//AverageRating      float32   `json:"averageRating"`
	//RatingCount        uint32    `json:"ratingCount"`
	//ModeratorIDs       []string  `json:"moderators"`
	//AcceptedCurrencies []string  `json:"acceptedCurrencies"`
	//CoinType           string    `json:"coinType"`

	ListingBytes   []byte `json:"-"`
	ListingVersion uint32 `json:"-"`
}

// ListingMetadata -
type ListingMetadata struct {
	Version uint `json:"version"`
}

// UnmarshalJSONListing - unmarshal listing
func UnmarshalJSONListing(data []byte) (*Listing, error) {
	var l IndividualListingContainer
	var v interface{}
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("unmarshal listing: %s", err.Error())
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("unmarshal listing: %s", err.Error())
	}
	listingData, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unmarshal listing: %s", "incorrect data")
	}
	listing, ok := listingData["listing"]
	if !ok {
		return nil, fmt.Errorf("unmarshal listing: %s", "incorrect data")
	}
	out, _ := json.Marshal(listing)
	l.Listing.ListingBytes = out
	l.Listing.ListingVersion = uint32(l.Metadata.Version)
	return &(l.Listing), nil
}

// GetTitle - return listing title
func (l *Listing) GetTitle() (string, error) {
	type title struct {
		Item struct {
			Title string `json:"title"`
		} `json:"item"`
	}
	var t title
	err := json.Unmarshal(l.ListingBytes, &t)
	if err != nil {
		return "", err
	}
	return t.Item.Title, nil
}

// GetSlug - return listing slug
func (l *Listing) GetSlug() (string, error) {
	type slug struct {
		Slug string `json:"slug"`
	}
	var s slug
	err := json.Unmarshal(l.ListingBytes, &s)
	if err != nil {
		return "", err
	}
	return s.Slug, nil
}

// GetPrice - return listing price
func (l *Listing) GetPrice() (CurrencyValue, error) {
	retVal := CurrencyValue{}
	type contractType struct {
		Metadata struct {
			ContractType string `json:"contractType"`
		} `json:"metadata"`
	}
	var ct contractType
	err := json.Unmarshal(l.ListingBytes, &ct)
	if err != nil {
		return retVal, err
	}
	switch l.ListingVersion {
	case 3, 4:
		{
			if ct.Metadata.ContractType == "CRYPTOCURRENCY" {
				retVal.Amount = big.NewInt(0)
				type coinType struct {
					Metadata struct {
						CoinType string `json:"coinType"`
					} `json:"metadata"`
				}
				var c coinType
				json.Unmarshal(l.ListingBytes, &c)
				retVal.Currency = &CurrencyDefinition{
					Code:         CurrencyCode(c.Metadata.CoinType),
					Divisibility: 8,
				}
			} else {
				type price struct {
					Item struct {
						Price int64 `json:"price"`
					} `json:"item"`
				}
				var p price
				json.Unmarshal(l.ListingBytes, &p)
				retVal.Amount = big.NewInt(p.Item.Price)
				type pricingCurrency struct {
					Metadata struct {
						PricingCurrency string `json:"pricingCurrency"`
					} `json:"metadata"`
				}
				var pc pricingCurrency
				json.Unmarshal(l.ListingBytes, &pc)
				retVal.Currency = &CurrencyDefinition{
					Code:         CurrencyCode(pc.Metadata.PricingCurrency),
					Divisibility: 8,
				}
			}
		}
	case 5:
		{
			type price struct {
				Item struct {
					Price struct {
						Currency struct {
							Code         string `json:"code"`
							Divisibility uint   `json:"divisibility"`
						} `json:"currency"`
						Amount string `json:"amount"`
					} `json:"price"`
				} `json:"item"`
			}
			var p price
			json.Unmarshal(l.ListingBytes, &p)
			retVal.Amount, _ = new(big.Int).SetString(p.Item.Price.Amount, 10)
			retVal.Currency = &CurrencyDefinition{
				Code:         CurrencyCode(p.Item.Price.Currency.Code),
				Divisibility: p.Item.Price.Currency.Divisibility,
			}
		}
	}
	return retVal, nil
}
