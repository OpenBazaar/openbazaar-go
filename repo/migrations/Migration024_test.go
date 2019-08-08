package migrations_test

import (
	"database/sql"
	"io/ioutil"
	"os"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func TestMigration024(t *testing.T) {
	var (
		basePath          = schema.GenerateTempPath()
		testRepoPath, err = schema.OpenbazaarPathTransform(basePath, true)
	)
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
		schemaPath   = appSchema.DataPathJoin("repover")

		schemaSQL         = "pragma key = 'foobarbaz';"
		selectMessagesSQL = "select * from messages;"
		//setupSQL          = strings.Join([]string{
		//	schemaSQL,
		//}, " ")
	)

	// create schema version file
	if err = ioutil.WriteFile(schemaPath, []byte("23"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// execute migration up
	m := migrations.Migration024{}
	if err := m.Up(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(schemaSQL); err != nil {
		t.Fatal(err)
	}

	// assert repo version updated
	if err = appSchema.VerifySchemaVersion("25"); err != nil {
		t.Fatal(err)
	}

	// verify change was applied properly
	_, err = db.Exec(selectMessagesSQL)
	if err != nil {
		t.Fatal(err)
	}

	// execute migration down
	if err := m.Down(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	// assert repo version reverted
	if err = appSchema.VerifySchemaVersion("24"); err != nil {
		t.Fatal(err)
	}

	// verify change was reverted properly
	_, err = db.Exec(selectMessagesSQL)
	if err == nil {
		t.Fatal(err)
	}

}
