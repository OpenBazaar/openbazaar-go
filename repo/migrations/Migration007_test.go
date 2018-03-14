package migrations

import (
	"database/sql"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/schema"
)

func TestMigration007(t *testing.T) {

	// Setup
	basePath := schema.GenerateTempPath()
	testRepoPath, err := schema.OpenbazaarPathTransform(basePath, true)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := schema.NewCustomSchemaManager(schema.SchemaContext{DataPath: testRepoPath, TestModeEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = paths.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer paths.DestroySchemaDirectories()
	var (
		databasePath = paths.DatastorePath()
		schemaPath   = paths.DataPathJoin("repover")

		schemaSql     = "PRAGMA key = 'foobarbaz';"
		insertCaseSQL = "INSERT INTO cases (caseID, state, read, timestamp, buyerOpened, claim, buyerPayoutAddress, vendorPayoutAddress) VALUES (?,?,?,?,?,?,?,?);"
		selectCaseSQL = "SELECT caseID, lastNotifiedAt FROM cases WHERE caseID = ?;"

		caseID = "caseID"
	)

	// Setup Cases datastore
	dbSetupSql := strings.Join([]string{
		schemaSql,
		migration007_casesCreateSQL,
		insertCaseSQL,
	}, " ")
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(dbSetupSql,
		caseID, // dispute case id
		int(0), // dispute OrderState
		0,      // dispute read bool
		int(time.Now().Unix()), // dispute timestamp
		0,           // dispute buyerOpened bool
		"claimtext", // dispute claim text
		"",          // dispute buyerPayoutAddres
		"",          // dispute vendorPayoutAddres
	)
	if err != nil {
		t.Fatal(err)
	}

	// Create schema version file
	if err = ioutil.WriteFile(schemaPath, []byte("7"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// Execute Migration Up
	migration := Migration007{}
	if err := migration.Up(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	// Assert repo version updated
	if err = paths.VerifySchemaVersion("8"); err != nil {
		t.Fatal(err)
	}

	caseRows, err := db.Query(selectCaseSQL, caseID)
	if err != nil {
		t.Fatal(err)
	}

	// Assert lastNotifiedAt column properties
	var lastNotifiedAtColumnExists bool
	columns, err := caseRows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range columns {
		if c.Name() == "lastNotifiedAt" {
			lastNotifiedAtColumnExists = true
		}
	}
	if lastNotifiedAtColumnExists == false {
		t.Error("Expected lastNotifiedAt column to exist and not be nullable")
	}

	// Assert lastNotifiedAt column is set to (approx) the same time the migration
	// was executed
	var actualCase struct {
		CaseId         string
		LastNotifiedAt int64
	}

	for caseRows.Next() {
		err := caseRows.Scan(&actualCase.CaseId, &actualCase.LastNotifiedAt)
		if err != nil {
			t.Error(err)
		}
		if actualCase.CaseId != caseID {
			t.Error("Unexpected case ID returned")
		}
		timeSinceMigration := time.Now().Sub(time.Unix(actualCase.LastNotifiedAt, 0))
		if timeSinceMigration > (time.Duration(2) * time.Second) {
			t.Errorf("Expected lastNotifiedAt to be set within the last 2 seconds, but was set %s ago", timeSinceMigration)
		}
	}
	caseRows.Close()
	db.Close()

	// Execute Migration Down
	if err = migration.Down(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	// Assert lastNotifiedAt column properties
	db, err = sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Query("update cases set lastNotifiedAt = ? where caseID = ?;", 0, caseID)
	if err == nil {
		t.Error("Expected lastNotifiedAt update to fail")
		t.Fatal(err)
	}

	if err = paths.VerifySchemaVersion("7"); err != nil {
		t.Error(err)
	}
	db.Close()

}
