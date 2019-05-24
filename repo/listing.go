package repo

import (
	"encoding/json"
	"fmt"
)

type IndividualListingContainer struct {
	Listing `json:"listing"`
}

// Listing represents a trade offer which can be accepted by another
// party on the OpenBazaar network
type Listing struct {
	Metadata ListingMetadata `json:"metadata"`
	//Hash               string    `json:"hash"`
	//Slug               string    `json:"slug"`
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
}

type ListingMetadata struct {
	Version uint `json:"version"`
}

func UnmarshalJSONListing(data []byte) (*Listing, error) {
	var l IndividualListingContainer
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("unmarshal listing: %s", err.Error())
	}
	return &(l.Listing), nil
}
