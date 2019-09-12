package migrations

import (
	"github.com/OpenBazaar/openbazaar-go/schema"
	"testing"
)

func TestMigration025(t *testing.T) {
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
		//repoVerPath   = appSchema.DataPathJoin("repover")

		column    = "needsSync"
		selectSQL = "select orderID from sales where " + column + "=1"
	)

	if err := appSchema.InitializeDatabase(); err != nil {
		t.Fatal(err)
	}

	migration := Migration025{}
	if err := migration.Up(appSchema.DataPath(), "", true); err != nil {
		t.Fatal(err)
	}

	db, err := appSchema.OpenDatabase()
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(selectSQL)
	expectedErr := "no such column: " + column
	if err == nil || err.Error() != expectedErr {
		t.Errorf("Expected %s got %s", expectedErr, err)
	}

	// assert repo version reverted
	if err = appSchema.VerifySchemaVersion("26"); err != nil {
		t.Fatal(err)
	}

	if err := migration.Down(appSchema.DataPath(), "", true); err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(selectSQL)
	if err != nil {
		t.Errorf("Expected nil error got %s", err)
	}

	// assert repo version reverted
	if err = appSchema.VerifySchemaVersion("25"); err != nil {
		t.Fatal(err)
	}
}
