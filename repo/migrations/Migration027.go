package migrations

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/core/mock"
)

type Migration027 struct{ UpdateListingHash }

// UpdateListingHash will update the hashes of each listing in the listing index with
// the newest hash format.
type UpdateListingHash struct{}

type UpdateListingHash_Price struct {
	CurrencyCode string  `json:"currencyCode"`
	Amount       uint64  `json:"amount"`
	Modifier     float32 `json:"modifier"`
}
type UpdateListingHash_Thumbnail struct {
	Tiny   string `json:"tiny"`
	Small  string `json:"small"`
	Medium string `json:"medium"`
}

type UpdateListingHash_ListingData struct {
	Hash               string                      `json:"hash"`
	Slug               string                      `json:"slug"`
	Title              string                      `json:"title"`
	Categories         []string                    `json:"categories"`
	NSFW               bool                        `json:"nsfw"`
	ContractType       string                      `json:"contractType"`
	Description        string                      `json:"description"`
	Thumbnail          UpdateListingHash_Thumbnail `json:"thumbnail"`
	Price              UpdateListingHash_Price     `json:"price"`
	ShipsTo            []string                    `json:"shipsTo"`
	FreeShipping       []string                    `json:"freeShipping"`
	Language           string                      `json:"language"`
	AverageRating      float32                     `json:"averageRating"`
	RatingCount        uint32                      `json:"ratingCount"`
	ModeratorIDs       []string                    `json:"moderators"`
	AcceptedCurrencies []string                    `json:"acceptedCurrencies"`
	CoinType           string                      `json:"coinType"`
}

func (UpdateListingHash) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	listingsFilePath := path.Join(repoPath, "root", "listings.json")

	// Non-vendors might not have an listing.json and we don't want to error here if that's the case
	indexExists := true
	if _, err := os.Stat(listingsFilePath); os.IsNotExist(err) {
		indexExists = false
	}

	if indexExists {
		var listingIndex []UpdateListingHash_ListingData
		listingsJSON, err := ioutil.ReadFile(listingsFilePath)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(listingsJSON, &listingIndex); err != nil {
			return err
		}
		n, err := coremock.NewMockNode()
		if err != nil {
			return err
		}
		for i, listing := range listingIndex {
			hash, err := ipfs.GetHashOfFile(n, path.Join(repoPath, "root", "listings", listing.Slug+".json"))
			if err != nil {
				return err
			}

			listingIndex[i].Hash = hash
		}
		migratedJSON, err := json.MarshalIndent(&listingIndex, "", "    ")
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(listingsFilePath, migratedJSON, os.ModePerm)
		if err != nil {
			return err
		}
	}

	return writeRepoVer(repoPath, 27)
}

func (UpdateListingHash) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	// Down migration is a no-op (outside of updating the version)
	// We can't calculate the old style hash format anymore.
	return writeRepoVer(repoPath, 26)
}
