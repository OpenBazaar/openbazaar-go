package migrations

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
)

// migration011ModeratorsKey is the key we want to manipulate in the JSON
const migration011ModeratorsKey = "moderators"

type Migration011 struct{}

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
		listingRecords   []map[string]interface{}
		listingsFilePath = path.Join(repoPath, "root", "listings.json")
	)

	// Don't do anything if no listing index exists
	if _, err := os.Stat(listingsFilePath); os.IsNotExist(err) {
		return writeRepoVer(repoPath, 12)
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
		if listing[migration011ModeratorsKey] != nil {
			continue
		}

		iSlug := listing["slug"]
		slug, ok := iSlug.(string)
		if !ok {
			return errors.New("'slug' is not a string")
		}

		listingFilePath := path.Join(repoPath, "root", "listings", slug+".json")
		listingJSON, err = ioutil.ReadFile(listingFilePath)
		if err != nil {
			return err
		}

		listingRecord := &Migration011_listing{}
		if err = json.Unmarshal(listingJSON, &listingRecord); err != nil {
			return err
		}
		listing[migration011ModeratorsKey] = listingRecord.Listing.ModeratorIDs
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
