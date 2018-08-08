package migrations

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
)

type Migration011 struct{}

type Migration011_listingIndexListing struct {
	Hash         string   `json:"hash"`
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Categories   []string `json:"categories"`
	NSFW         bool     `json:"nsfw"`
	CoinType     string   `json:"coinType"`
	ContractType string   `json:"contractType"`
	Description  string   `json:"description"`
	Thumbnail    struct {
		Tiny   string `json:"tiny"`
		Small  string `json:"small"`
		Medium string `json:"medium"`
	} `json:"thumbnail"`
	Price struct {
		CurrencyCode string `json:"currencyCode"`
		Amount       uint64 `json:"amount"`
	} `json:"price"`
	ShipsTo            []string `json:"shipsTo"`
	FreeShipping       []string `json:"freeShipping"`
	Language           string   `json:"language"`
	AverageRating      float32  `json:"averageRating"`
	RatingCount        uint32   `json:"ratingCount"`
	AcceptedCurrencies []string `json:"acceptedCurrencies"`
	ModeratorIDs       []string `json:"moderators"`
}

type Migration011_listing struct {
	Listing Migration011_listing_listing `json:"listing"`
}
type Migration011_listing_listing struct {
	ModeratorIDs []string `json:"moderators"`
}

func (Migration011) Up(repoPath string, dbPassword string, testnet bool) error {
	var (
		err              error
		listingJSON      []byte
		listingsJSON     []byte
		listingRecords   []*Migration011_listingIndexListing
		listingsFilePath = path.Join(repoPath, "root", "listings.json")
	)

	// Don't do anything if no listing index exists
	if _, err := os.Stat(listingsFilePath); os.IsNotExist(err) {
		return nil
	}

	// Load index
	listingsJSON, err = ioutil.ReadFile(listingsFilePath)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(listingsJSON, &listingRecords); err != nil {
		return err
	}

	// Iterate over listing. If a listing is missing moderators then load the full
	// listing file and populate the listing index abstract
	for _, listing := range listingRecords {
		if len(listing.ModeratorIDs) > 0 {
			continue
		}

		listingFilePath := path.Join(repoPath, "root", "listings", listing.Slug+".json")
		listingJSON, err = ioutil.ReadFile(listingFilePath)
		if err != nil {
			return err
		}

		listingRecord := &Migration011_listing{}
		if err = json.Unmarshal(listingJSON, &listingRecord); err != nil {
			return err
		}
		listing.ModeratorIDs = listingRecord.Listing.ModeratorIDs
	}

	// Write updated index back to disk
	listingsJSON, err = json.MarshalIndent(listingRecords, "", "    ")
	if err != nil {
		return err
	}

	ioutil.WriteFile(listingsFilePath, listingsJSON, os.ModePerm)
	if err != nil {
		return err
	}

	return writeRepoVer(repoPath, 12)
}

func (Migration011) Down(repoPath string, dbPassword string, testnet bool) error {
	return writeRepoVer(repoPath, 11)
}
