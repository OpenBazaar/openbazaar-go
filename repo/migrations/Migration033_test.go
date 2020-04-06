package migrations_test

import (
	"database/sql"
	"io/ioutil"
	"os"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func TestMigration033(t *testing.T) {
	var (
		basePath          = schema.GenerateTempPath()
		testRepoPath, err = schema.OpenbazaarPathTransform(basePath, true)

		testMigration033SchemaStmts = []string{
			"DROP TABLE IF EXISTS messages;",
			migrations.MigrationCreateMessagesAM09MessagesCreateSQLDown,
		}
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

	if err := appSchema.InitializeDatabase(); err != nil {
		t.Fatal(err)
	}

	var (
		databasePath = appSchema.DatabasePath()
		schemaPath   = appSchema.DataPathJoin("repover")

		insertSQL = "insert into messages(messageID, err, received_at) values(?,?,?)"
	)

	// create schema version file
	if err = ioutil.WriteFile(schemaPath, []byte("33"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, stmt := range testMigration033SchemaStmts {
		_, err := db.Exec(stmt)
		if err != nil {
			t.Fatal(err)
		}
	}

	// execute migration up
	m := migrations.Migration033{}
	if err := m.Up(testRepoPath, "", true); err != nil {
		t.Fatal(err)
	}

	// assert repo version updated
	if err = appSchema.VerifySchemaVersion("34"); err != nil {
		t.Fatal(err)
	}

	// verify change was applied properly
	_, err = db.Exec(insertSQL, "abc", "", 0)
	if err != nil {
		t.Fatal(err)
	}

	// execute migration down
	if err := m.Down(testRepoPath, "", true); err != nil {
		t.Fatal(err)
	}

	// assert repo version reverted
	if err = appSchema.VerifySchemaVersion("33"); err != nil {
		t.Fatal(err)
	}

	// verify change was reverted properly
	_, err = db.Exec(insertSQL, "abc", "", 0)
	if err == nil {
		t.Fatal(err)
	}
}
