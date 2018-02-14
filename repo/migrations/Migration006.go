package migrations

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/OpenBazaar/openbazaar-go/util"
)

type migration006_configRecord struct {
	StoreModerators []string `json:"storeModerators"`
}

type migration006_price struct {
	CurrencyCode string `json:"currencyCode"`
	Amount       uint64 `json:"amount"`
}
type migration006_thumbnail struct {
	Tiny   string `json:"tiny"`
	Small  string `json:"small"`
	Medium string `json:"medium"`
}

type migration006_listingDataBeforeMigration struct {
	Hash          string                 `json:"hash"`
	Slug          string                 `json:"slug"`
	Title         string                 `json:"title"`
	Categories    []string               `json:"categories"`
	NSFW          bool                   `json:"nsfw"`
	ContractType  string                 `json:"contractType"`
	Description   string                 `json:"description"`
	Thumbnail     migration006_thumbnail `json:"thumbnail"`
	Price         migration006_price     `json:"price"`
	ShipsTo       []string               `json:"shipsTo"`
	FreeShipping  []string               `json:"freeShipping"`
	Language      string                 `json:"language"`
	AverageRating float32                `json:"averageRating"`
	RatingCount   uint32                 `json:"ratingCount"`
}

type migration006_listingDataAfterMigration struct {
	Hash          string                 `json:"hash"`
	Slug          string                 `json:"slug"`
	Title         string                 `json:"title"`
	Categories    []string               `json:"categories"`
	NSFW          bool                   `json:"nsfw"`
	ContractType  string                 `json:"contractType"`
	Description   string                 `json:"description"`
	Thumbnail     migration006_thumbnail `json:"thumbnail"`
	Price         migration006_price     `json:"price"`
	ShipsTo       []string               `json:"shipsTo"`
	FreeShipping  []string               `json:"freeShipping"`
	Language      string                 `json:"language"`
	AverageRating float32                `json:"averageRating"`
	RatingCount   uint32                 `json:"ratingCount"`
	ModeratorIDs  []string               `json:"moderatorIDs"`
}

type migration006 struct{}

func (migration006) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	paths, err := util.NewCustomSchemaManager(util.SchemaContext{
		RootPath:        repoPath,
		TestModeEnabled: testnetEnabled,
	})

	// Get ModeratorIDs
	db, err := sql.Open("sqlite3", paths.DatastorePath())
	if err != nil {
		return err
	}
	if databasePassword != "" {
		p := fmt.Sprintf("PRAGMA key = '%s';", databasePassword)
		_, err := db.Exec(p)
		if err != nil {
			return err
		}
	}
	var (
		configJSON   []byte
		configRecord migration006_configRecord
	)
	configQuery := db.QueryRow("SELECT value FROM config WHERE key = 'settings' LIMIT 1")
	if err = configQuery.Scan(&configJSON); err != nil {
		return err
	}
	if err = json.Unmarshal(configJSON, &configRecord); err != nil {
		return err
	}

	// Listing transformation
	var (
		listingJSON     []byte
		listingRecords  []migration006_listingDataBeforeMigration
		migratedRecords []migration006_listingDataAfterMigration
	)
	listingJSON, err = ioutil.ReadFile(paths.RootPathJoin("listings.json"))
	if err != nil {
		return err
	}
	if err = json.Unmarshal(listingJSON, &listingRecords); err != nil {
		return err
	}

	for _, listing := range listingRecords {
		migratedRecords = append(migratedRecords, migration006_listingDataAfterMigration{
			Hash:          listing.Hash,
			Slug:          listing.Slug,
			Title:         listing.Title,
			Categories:    listing.Categories,
			NSFW:          listing.NSFW,
			ContractType:  listing.ContractType,
			Description:   listing.Description,
			Thumbnail:     listing.Thumbnail,
			Price:         listing.Price,
			ShipsTo:       listing.ShipsTo,
			FreeShipping:  listing.FreeShipping,
			Language:      listing.Language,
			AverageRating: listing.AverageRating,
			RatingCount:   listing.RatingCount,
			ModeratorIDs:  configRecord.StoreModerators,
		})
	}
	if listingJSON, err = json.Marshal(migratedRecords); err != nil {
		return err
	}
	err = ioutil.WriteFile(paths.RootPathJoin("listings.json"), listingJSON, os.ModePerm)
	if err != nil {
		return err
	}

	// Bump schema version
	err = ioutil.WriteFile(paths.RootPathJoin("repover"), []byte("7"), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func (migration006) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	paths, err := util.NewCustomSchemaManager(util.SchemaContext{
		RootPath:        repoPath,
		TestModeEnabled: testnetEnabled,
	})
	if err != nil {
		return err
	}

	// Listing transformation
	var (
		listingJSON     []byte
		listingRecords  []migration006_listingDataAfterMigration
		migratedRecords []migration006_listingDataBeforeMigration
	)
	listingJSON, err = ioutil.ReadFile(paths.RootPathJoin("listings.json"))
	if err != nil {
		return err
	}
	if err = json.Unmarshal(listingJSON, &listingRecords); err != nil {
		return err
	}

	for _, listing := range listingRecords {
		migratedRecords = append(migratedRecords, migration006_listingDataBeforeMigration{
			Hash:          listing.Hash,
			Slug:          listing.Slug,
			Title:         listing.Title,
			Categories:    listing.Categories,
			NSFW:          listing.NSFW,
			ContractType:  listing.ContractType,
			Description:   listing.Description,
			Thumbnail:     listing.Thumbnail,
			Price:         listing.Price,
			ShipsTo:       listing.ShipsTo,
			FreeShipping:  listing.FreeShipping,
			Language:      listing.Language,
			AverageRating: listing.AverageRating,
			RatingCount:   listing.RatingCount,
		})
	}
	if listingJSON, err = json.Marshal(migratedRecords); err != nil {
		return err
	}
	err = ioutil.WriteFile(paths.RootPathJoin("listings.json"), listingJSON, os.ModePerm)
	if err != nil {
		return err
	}

	// Revert schema version
	err = ioutil.WriteFile(paths.RootPathJoin("repover"), []byte("6"), os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}
