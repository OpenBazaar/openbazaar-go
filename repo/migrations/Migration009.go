package migrations

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	_ "github.com/mutecomm/go-sqlcipher"
)

type Migration009 struct{}

type Migration009_price struct {
	CurrencyCode string `json:"currencyCode"`
	Amount       uint64 `json:"amount"`
}
type Migration009_thumbnail struct {
	Tiny   string `json:"tiny"`
	Small  string `json:"small"`
	Medium string `json:"medium"`
}

type Migration009_listingDataBeforeMigration struct {
	Hash          string                 `json:"hash"`
	Slug          string                 `json:"slug"`
	Title         string                 `json:"title"`
	Categories    []string               `json:"categories"`
	NSFW          bool                   `json:"nsfw"`
	CoinType      string                 `json:"coinType"`
	ContractType  string                 `json:"contractType"`
	Description   string                 `json:"description"`
	Thumbnail     Migration009_thumbnail `json:"thumbnail"`
	Price         Migration009_price     `json:"price"`
	ShipsTo       []string               `json:"shipsTo"`
	FreeShipping  []string               `json:"freeShipping"`
	Language      string                 `json:"language"`
	AverageRating float32                `json:"averageRating"`
	RatingCount   uint32                 `json:"ratingCount"`
	ModeratorIDs  []string               `json:"moderators"`
}

type Migration009_listingDataAfterMigration struct {
	Hash          string                 `json:"hash"`
	Slug          string                 `json:"slug"`
	Title         string                 `json:"title"`
	Categories    []string               `json:"categories"`
	NSFW          bool                   `json:"nsfw"`
	CoinType      string                 `json:"coinType"`
	ContractType  string                 `json:"contractType"`
	Description   string                 `json:"description"`
	Thumbnail     Migration009_thumbnail `json:"thumbnail"`
	Price         Migration009_price     `json:"price"`
	ShipsTo       []string               `json:"shipsTo"`
	FreeShipping  []string               `json:"freeShipping"`
	Language      string                 `json:"language"`
	AverageRating float32                `json:"averageRating"`
	RatingCount   uint32                 `json:"ratingCount"`
	ModeratorIDs  []string               `json:"moderators"`

	// Adding AcceptedCurrencies
	AcceptedCurrencies []string `json:"acceptedCurrencies"`
}

type Migration009_listing struct {
	Listing Migration009_listing_listing `json:"listing"`
}
type Migration009_listing_listing struct {
	Metadata Migration009_listing_listing_metadata `json:"metadata"`
}
type Migration009_listing_listing_metadata struct {
	AcceptedCurrencies []string `json:"acceptedCurrencies`
}

const (
	Migration009CreatePreviousCasesTable     = "create table cases (caseID text primary key not null, buyerContract blob, vendorContract blob, buyerValidationErrors blob, vendorValidationErrors blob, buyerPayoutAddress text, vendorPayoutAddress text, buyerOutpoints blob, vendorOutpoints blob, state integer, read integer, timestamp integer, buyerOpened integer, claim text, disputeResolution blob, lastDisputeExpiryNotifiedAt integer not null default 0);"
	Migration009CreatePreviousSalesTable     = "create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, needsSync integer, lastDisputeTimeoutNotifiedAt integer not null default 0);"
	Migration009CreatePreviousSalesIndex     = "create index index_sales on sales (paymentAddr, timestamp);"
	Migration009CreatePreviousPurchasesTable = "create table purchases (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, vendorID text, vendorHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, lastDisputeTimeoutNotifiedAt integer not null default 0, lastDisputeExpiryNotifiedAt integer not null default 0, disputedAt integer not null default 0);"
)

func (Migration009) Up(repoPath string, dbPassword string, testnet bool) (err error) {
	db, err := OpenDB(repoPath, dbPassword, testnet)
	if err != nil {
		return err
	}

	// Update DB schema
	err = withTransaction(db, func(tx *sql.Tx) error {
		for _, stmt := range []string{
			"ALTER TABLE cases ADD COLUMN coinType text DEFAULT '';",
			"ALTER TABLE sales ADD COLUMN coinType text DEFAULT '';",
			"ALTER TABLE purchases ADD COLUMN coinType text DEFAULT '';",
			"ALTER TABLE cases ADD COLUMN paymentCoin text DEFAULT '';",
			"ALTER TABLE sales ADD COLUMN paymentCoin text DEFAULT '';",
			"ALTER TABLE purchases ADD COLUMN paymentCoin text DEFAULT '';",
		} {
			_, err := tx.Exec(stmt)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Update repover now that the schema is changed
	err = writeRepoVer(repoPath, 10)
	if err != nil {
		return err
	}

	// Update DB data on a best effort basis
	err = migration009UpdateTablesCoins(db, "cases", "caseID", "COALESCE(buyerContract, vendorContract) AS contract")
	if err != nil {
		return err
	}

	err = migration009UpdateTablesCoins(db, "sales", "orderID", "contract")
	if err != nil {
		return err
	}

	err = migration009UpdateTablesCoins(db, "purchases", "orderID", "contract")
	if err != nil {
		return err
	}

	err = migration009MigrateListingsIndexUp(repoPath)
	if err != nil {
		return err
	}

	return nil
}

func (Migration009) Down(repoPath string, dbPassword string, testnet bool) error {
	db, err := OpenDB(repoPath, dbPassword, testnet)
	if err != nil {
		return err
	}

	err = withTransaction(db, func(tx *sql.Tx) error {
		for _, stmt := range []string{
			"ALTER TABLE cases RENAME TO temp_cases;",
			Migration009CreatePreviousCasesTable,
			"INSERT INTO cases SELECT caseID, buyerContract, vendorContract, buyerValidationErrors, vendorValidationErrors, buyerPayoutAddress, vendorPayoutAddress, buyerOutpoints, vendorOutpoints, state, read, timestamp, buyerOpened, claim, disputeResolution, lastDisputeExpiryNotifiedAt FROM temp_cases;",
			"DROP TABLE temp_cases;",

			"ALTER TABLE sales RENAME TO temp_sales;",
			Migration009CreatePreviousSalesTable,
			Migration009CreatePreviousSalesIndex,
			"INSERT INTO sales SELECT orderID, contract, state, read, timestamp, total, thumbnail, buyerID, buyerHandle, title, shippingName, shippingAddress, paymentAddr, funded, transactions, needsSync, lastDisputeTimeoutNotifiedAt FROM temp_sales;",
			"DROP TABLE temp_sales;",

			"ALTER TABLE purchases RENAME TO temp_purchases;",
			Migration009CreatePreviousPurchasesTable,
			"INSERT INTO purchases SELECT orderID, contract, state, read, timestamp, total, thumbnail, vendorID, vendorHandle, title, shippingName, shippingAddress, paymentAddr, funded, transactions, lastDisputeTimeoutNotifiedAt, lastDisputeExpiryNotifiedAt, disputedAt FROM temp_purchases;",
			"DROP TABLE temp_purchases;",
		} {
			_, err := tx.Exec(stmt)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = writeRepoVer(repoPath, 9)
	if err != nil {
		return err
	}

	err = migration009MigrateListingsIndexDown(repoPath)
	if err != nil {
		return err
	}

	return nil
}

func migration009UpdateTablesCoins(db *sql.DB, table string, idColumn string, contractColumn string) error {
	type coinset struct {
		paymentCoin string
		coinType    string
	}

	// Get all records for table and store the coinset for each entry
	rows, err := db.Query("SELECT " + idColumn + ", " + contractColumn + " FROM " + table + ";")
	if err != nil {
		return err
	}
	defer rows.Close()

	coinsToSet := map[string]coinset{}
	for rows.Next() {
		var orderID, marshaledContract string

		err = rows.Scan(&orderID, &marshaledContract)
		if err != nil {
			return err
		}

		if marshaledContract == "" {
			continue
		}

		contract := &pb.RicardianContract{}
		if err := jsonpb.UnmarshalString(marshaledContract, contract); err != nil {
			return err
		}

		coinsToSet[orderID] = coinset{
			coinType:    coinTypeForContract(contract),
			paymentCoin: paymentCoinForContract(contract),
		}
	}

	// Update each row with the coins
	err = withTransaction(db, func(tx *sql.Tx) error {
		for id, coins := range coinsToSet {
			_, err := tx.Exec(
				"UPDATE "+table+" SET coinType = ?, paymentCoin = ? WHERE "+idColumn+" = ?",
				coins.coinType,
				coins.paymentCoin,
				id)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func paymentCoinForContract(contract *pb.RicardianContract) string {
	paymentCoin := contract.BuyerOrder.Payment.Coin
	if paymentCoin != "" {
		return paymentCoin
	}

	if len(contract.VendorListings[0].Metadata.AcceptedCurrencies) > 0 {
		paymentCoin = contract.VendorListings[0].Metadata.AcceptedCurrencies[0]
	}

	return paymentCoin
}

func coinTypeForContract(contract *pb.RicardianContract) string {
	coinType := ""

	if len(contract.VendorListings) > 0 {
		coinType = contract.VendorListings[0].Metadata.CoinType
	}

	return coinType
}

func migration009MigrateListingsIndexUp(repoPath string) error {
	listingsFilePath := path.Join(repoPath, "root", "listings.json")

	if _, err := os.Stat(listingsFilePath); os.IsNotExist(err) {
		return nil
	}

	var (
		err             error
		paymentCoin     string
		listingJSON     []byte
		listingsJSON    []byte
		listingRecord   Migration009_listing
		listingRecords  []Migration009_listingDataBeforeMigration
		migratedRecords []Migration009_listingDataAfterMigration
	)

	listingsJSON, err = ioutil.ReadFile(listingsFilePath)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(listingsJSON, &listingRecords); err != nil {
		return err
	}

	for _, listing := range listingRecords {
		if paymentCoin == "" {
			listingFilePath := path.Join(repoPath, "root", "listings", listing.Slug+".json")
			listingJSON, err = ioutil.ReadFile(listingFilePath)
			if err != nil {
				return err
			}
			if err = json.Unmarshal(listingJSON, &listingRecord); err != nil {
				return err
			}
			paymentCoin = listingRecord.Listing.Metadata.AcceptedCurrencies[0]
		}

		migratedRecords = append(migratedRecords, Migration009_listingDataAfterMigration{
			Hash:               listing.Hash,
			Slug:               listing.Slug,
			Title:              listing.Title,
			Categories:         listing.Categories,
			NSFW:               listing.NSFW,
			ContractType:       listing.ContractType,
			Description:        listing.Description,
			Thumbnail:          listing.Thumbnail,
			Price:              listing.Price,
			ShipsTo:            listing.ShipsTo,
			FreeShipping:       listing.FreeShipping,
			Language:           listing.Language,
			AverageRating:      listing.AverageRating,
			RatingCount:        listing.RatingCount,
			CoinType:           listing.CoinType,
			AcceptedCurrencies: []string{paymentCoin},
		})
	}
	if listingsJSON, err = json.MarshalIndent(migratedRecords, "", "    "); err != nil {
		return err
	}
	err = ioutil.WriteFile(listingsFilePath, listingsJSON, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

func migration009MigrateListingsIndexDown(repoPath string) error {
	listingsFilePath := path.Join(repoPath, "root", "listings.json")

	if _, err := os.Stat(listingsFilePath); os.IsNotExist(err) {
		return nil
	}

	var (
		err             error
		listingsJSON    []byte
		listingRecords  []Migration009_listingDataAfterMigration
		migratedRecords []Migration009_listingDataBeforeMigration
	)

	listingsJSON, err = ioutil.ReadFile(listingsFilePath)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(listingsJSON, &listingRecords); err != nil {
		return err
	}

	for _, listing := range listingRecords {
		migratedRecords = append(migratedRecords, Migration009_listingDataBeforeMigration{
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
			CoinType:      listing.CoinType,
		})
	}
	if listingsJSON, err = json.MarshalIndent(migratedRecords, "", "    "); err != nil {
		return err
	}
	err = ioutil.WriteFile(listingsFilePath, listingsJSON, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}
