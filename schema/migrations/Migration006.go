package migrations

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

type Migration006 struct{}

type Migration006_configRecord struct {
	StoreModerators []string `json:"storeModerators"`
}

type Migration006_price struct {
	CurrencyCode string `json:"currencyCode"`
	Amount       uint64 `json:"amount"`
}
type Migration006_thumbnail struct {
	Tiny   string `json:"tiny"`
	Small  string `json:"small"`
	Medium string `json:"medium"`
}

type Migration006_listingDataBeforeMigration struct {
	Hash          string                 `json:"hash"`
	Slug          string                 `json:"slug"`
	Title         string                 `json:"title"`
	Categories    []string               `json:"categories"`
	NSFW          bool                   `json:"nsfw"`
	ContractType  string                 `json:"contractType"`
	Description   string                 `json:"description"`
	Thumbnail     Migration006_thumbnail `json:"thumbnail"`
	Price         Migration006_price     `json:"price"`
	ShipsTo       []string               `json:"shipsTo"`
	FreeShipping  []string               `json:"freeShipping"`
	Language      string                 `json:"language"`
	AverageRating float32                `json:"averageRating"`
	RatingCount   uint32                 `json:"ratingCount"`
}

type Migration006_listingDataAfterMigration struct {
	Hash          string                 `json:"hash"`
	Slug          string                 `json:"slug"`
	Title         string                 `json:"title"`
	Categories    []string               `json:"categories"`
	NSFW          bool                   `json:"nsfw"`
	ContractType  string                 `json:"contractType"`
	Description   string                 `json:"description"`
	Thumbnail     Migration006_thumbnail `json:"thumbnail"`
	Price         Migration006_price     `json:"price"`
	ShipsTo       []string               `json:"shipsTo"`
	FreeShipping  []string               `json:"freeShipping"`
	Language      string                 `json:"language"`
	AverageRating float32                `json:"averageRating"`
	RatingCount   uint32                 `json:"ratingCount"`

	// Adding ModeratorIDs
	ModeratorIDs []string `json:"moderators"`
}

func (Migration006) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	var (
		databaseFilePath    string
		listingsFilePath    = path.Join(repoPath, "root", "listings.json")
		repoVersionFilePath = path.Join(repoPath, "repover")
	)
	if testnetEnabled {
		databaseFilePath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		databaseFilePath = path.Join(repoPath, "datastore", "mainnet.db")
	}

	// Non-vendors might not have an listing.json and we don't want to error here if that's the case
	indexExists := true
	if _, err := os.Stat(listingsFilePath); os.IsNotExist(err) {
		indexExists = false
	}

	if indexExists {
		// Get ModeratorIDs
		db, err := sql.Open("sqlite3", databaseFilePath)
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
			configJSON      []byte
			configRecord    Migration006_configRecord
			storeModerators []string
		)
		configQuery := db.QueryRow("SELECT value FROM config WHERE key = 'settings' LIMIT 1")
		err = configQuery.Scan(&configJSON)
		if err != nil {
			if err != sql.ErrNoRows {
				return err
			}
			storeModerators = make([]string, 0)
		} else {
			if err = json.Unmarshal(configJSON, &configRecord); err != nil {
				return err
			}
			storeModerators = configRecord.StoreModerators
		}

		// Listing transformation
		var (
			listingJSON     []byte
			listingRecords  []Migration006_listingDataBeforeMigration
			migratedRecords []Migration006_listingDataAfterMigration
		)
		listingJSON, err = ioutil.ReadFile(listingsFilePath)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(listingJSON, &listingRecords); err != nil {
			return err
		}

		for _, listing := range listingRecords {
			migratedRecords = append(migratedRecords, Migration006_listingDataAfterMigration{
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
				ModeratorIDs:  storeModerators,
			})
		}
		if listingJSON, err = json.Marshal(migratedRecords); err != nil {
			return err
		}
		err = ioutil.WriteFile(listingsFilePath, listingJSON, os.ModePerm)
		if err != nil {
			return err
		}
	}

	// Bump schema version
	err := ioutil.WriteFile(repoVersionFilePath, []byte("7"), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func (Migration006) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	var (
		listingsFilePath    = path.Join(repoPath, "root", "listings.json")
		repoVersionFilePath = path.Join(repoPath, "repover")
	)

	indexExists := true
	if _, err := os.Stat(listingsFilePath); os.IsNotExist(err) {
		indexExists = false
	}

	if indexExists {
		// Listing transformation
		var (
			listingJSON     []byte
			listingRecords  []Migration006_listingDataAfterMigration
			migratedRecords []Migration006_listingDataBeforeMigration
		)
		listingJSON, err := ioutil.ReadFile(listingsFilePath)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(listingJSON, &listingRecords); err != nil {
			return err
		}

		for _, listing := range listingRecords {
			migratedRecords = append(migratedRecords, Migration006_listingDataBeforeMigration{
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
		if listingJSON, err = json.MarshalIndent(migratedRecords, "", "    "); err != nil {
			return err
		}
		err = ioutil.WriteFile(listingsFilePath, listingJSON, os.ModePerm)
		if err != nil {
			return err
		}
	}

	// Revert schema version
	err := ioutil.WriteFile(repoVersionFilePath, []byte("6"), os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}
