package migrations

import (
	"encoding/json"
	"fmt"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/core/mock"
	"io/ioutil"
	"os"
	"path"
)

// Migration029 will update the hashes of each listing in the listing index with
// the newest hash format.
type Migration029 struct{}

type Migration029_Price struct {
	CurrencyCode string  `json:"currencyCode"`
	Amount       uint64  `json:"amount"`
	Modifier     float32 `json:"modifier"`
}
type Migration029_Thumbnail struct {
	Tiny   string `json:"tiny"`
	Small  string `json:"small"`
	Medium string `json:"medium"`
}

type Migration029_ListingData struct {
	Hash               string                 `json:"hash"`
	Slug               string                 `json:"slug"`
	Title              string                 `json:"title"`
	Categories         []string               `json:"categories"`
	NSFW               bool                   `json:"nsfw"`
	ContractType       string                 `json:"contractType"`
	Description        string                 `json:"description"`
	Thumbnail          Migration029_Thumbnail `json:"thumbnail"`
	Price              Migration029_Price     `json:"price"`
	ShipsTo            []string               `json:"shipsTo"`
	FreeShipping       []string               `json:"freeShipping"`
	Language           string                 `json:"language"`
	AverageRating      float32                `json:"averageRating"`
	RatingCount        uint32                 `json:"ratingCount"`
	ModeratorIDs       []string               `json:"moderators"`
	AcceptedCurrencies []string               `json:"acceptedCurrencies"`
	CoinType           string                 `json:"coinType"`
}

func (Migration029) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	listingsFilePath := path.Join(repoPath, "root", "listings.json")

	// Non-vendors might not have an listing.json and we don't want to error here if that's the case
	indexExists := true
	if _, err := os.Stat(listingsFilePath); os.IsNotExist(err) {
		indexExists = false
		fmt.Println(listingsFilePath)
	}

	if indexExists {
		var listingIndex []Migration029_ListingData
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

	return writeRepoVer(repoPath, 30)
}

func (Migration029) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	// Down migration is a no-op (outside of updating the version)
	// We can't calculate the old style hash format anymore.
	return writeRepoVer(repoPath, 29)
}
