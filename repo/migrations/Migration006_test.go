package migrations_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func TestMigration006(t *testing.T) {
	// Setup
	basePath := schema.GenerateTempPath()
	testRepoPath, err := schema.OpenbazaarPathTransform(basePath, true)
	if err != nil {
		t.Fatal(err)
	}
	appSchema, err := schema.NewCustomSchemaManager(schema.SchemaContext{DataPath: testRepoPath, TestModeEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = appSchema.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer appSchema.DestroySchemaDirectories()
	var (
		databasePath = appSchema.DatabasePath()
		listingPath  = appSchema.DataPathJoin("root", "listings.json")
		schemaPath   = appSchema.DataPathJoin("repover")

		schemaSql               = "PRAGMA key = 'foobarbaz';"
		configCreateSql         = "CREATE TABLE `config` (`key` text NOT NULL, `value` blob, PRIMARY KEY(`key`));"
		expectedStoreModerators = []string{
			"storemoderator_peerid_1",
			"storemoderator_peerid_2",
		}
		listingFixtures = []migrations.Migration006_listingDataBeforeMigration{
			{Hash: "Listing1"},
			{Hash: "Listing2"},
		}
		configRecord         = migrations.Migration006_configRecord{StoreModerators: expectedStoreModerators}
		insertConfigTemplate = "INSERT INTO config (key,value) VALUES ('settings','%s');"
	)

	// Create/persist settings record
	configJson, err := json.Marshal(configRecord)
	if err != nil {
		t.Fatal(err)
	}
	dbSetupSql := strings.Join([]string{
		schemaSql,
		configCreateSql,
		fmt.Sprintf(insertConfigTemplate, configJson),
	}, " ")
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(dbSetupSql)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Create listings.json file
	listingJSON, err := json.Marshal(listingFixtures)
	if err != nil {
		t.Fatal(err)
	}
	if err = ioutil.WriteFile(listingPath, listingJSON, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// Create schema version file
	if err = ioutil.WriteFile(schemaPath, []byte("6"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// Execute Migration Up
	migration := migrations.Migration006{}
	if err := migration.Up(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	// Migration Up Assertions
	if err = appSchema.VerifySchemaVersion("7"); err != nil {
		t.Fatal(err)
	}

	listingJSON, err = ioutil.ReadFile(listingPath)
	if err != nil {
		t.Fatal(err)
	}

	var migratedListings = make([]migrations.Migration006_listingDataAfterMigration, 2)
	err = json.Unmarshal(listingJSON, &migratedListings)
	if err != nil {
		t.Fatal(err)
	}

	if actualCount := len(migratedListings); actualCount != len(listingFixtures) {
		t.Fatalf("Unexpected number of listings returned. Actual count: %d", actualCount)
	}

	for _, listing := range migratedListings {
		actualModeratorIDs := make([]string, 0)
		for _, peerID := range listing.ModeratorIDs {
			actualModeratorIDs = append(actualModeratorIDs, peerID)
		}

		if !reflect.DeepEqual(expectedStoreModerators, actualModeratorIDs) {
			t.Fatalf("Expected moderator IDs were not equal\n\tExpected: %+v\n\tActual: %+v",
				expectedStoreModerators,
				actualModeratorIDs,
			)
		}
	}

	// Execute Migration Down
	if err = migration.Down(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	// Migration Down Assertions
	if err = appSchema.VerifySchemaVersion("6"); err != nil {
		t.Error(err)
	}

	listingJSON, err = ioutil.ReadFile(listingPath)
	if err != nil {
		t.Fatal(err)
	}

	migratedListings = make([]migrations.Migration006_listingDataAfterMigration, 2)
	err = json.Unmarshal(listingJSON, &migratedListings)
	if err != nil {
		t.Fatal(err)
	}

	if actualCount := len(migratedListings); actualCount != len(listingFixtures) {
		t.Fatalf("Unexpected number of listings returned. Actual count: %d", actualCount)
	}

	for _, listing := range migratedListings {
		if len(listing.ModeratorIDs) != 0 {
			t.Fatalf("Unexpected ModeratorIDs values: %+v", listing.ModeratorIDs)
		}
	}
}

func TestMigration006HandlesMissingSettings(t *testing.T) {

	// Setup
	basePath := schema.GenerateTempPath()
	testRepoPath, err := schema.OpenbazaarPathTransform(basePath, true)
	if err != nil {
		t.Fatal(err)
	}
	appSchema, err := schema.NewCustomSchemaManager(schema.SchemaContext{DataPath: testRepoPath, TestModeEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = appSchema.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer appSchema.DestroySchemaDirectories()
	var (
		databasePath = appSchema.DatabasePath()
		listingPath  = appSchema.DataPathJoin("root", "listings.json")
		schemaPath   = appSchema.DataPathJoin("repover")

		schemaSql       = "PRAGMA key = 'foobarbaz';"
		configCreateSql = "CREATE TABLE `config` (`key` text NOT NULL, `value` blob, PRIMARY KEY(`key`));"
		listingFixtures = []migrations.Migration006_listingDataBeforeMigration{
			{Hash: "Listing1"},
			{Hash: "Listing2"},
		}
	)

	// Create/persist settings record
	dbSetupSql := strings.Join([]string{
		schemaSql,
		configCreateSql,
	}, " ")
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(dbSetupSql)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Create listings.json file
	listingJSON, err := json.Marshal(listingFixtures)
	if err != nil {
		t.Fatal(err)
	}
	if err = ioutil.WriteFile(listingPath, listingJSON, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// Create schema version file
	if err = ioutil.WriteFile(schemaPath, []byte("6"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// Execute Migration Up
	migration := migrations.Migration006{}
	if err := migration.Up(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	// Migration Up Assertions
	if err = appSchema.VerifySchemaVersion("7"); err != nil {
		t.Fatal(err)
	}

	listingJSON, err = ioutil.ReadFile(listingPath)
	if err != nil {
		t.Fatal(err)
	}

	var migratedListings = make([]migrations.Migration006_listingDataAfterMigration, 2)
	err = json.Unmarshal(listingJSON, &migratedListings)
	if err != nil {
		t.Fatal(err)
	}

	if actualCount := len(migratedListings); actualCount != len(listingFixtures) {
		t.Fatalf("Unexpected number of listings returned. Actual count: %d", actualCount)
	}

	for _, listing := range migratedListings {
		if len(listing.ModeratorIDs) != 0 {
			t.Errorf("Expected StoreModerators to default to empty if settings are not initialized")
			t.Errorf("Actual: %+v", listing.ModeratorIDs)
		}
	}
}
