package migrations_test

import (
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
)

func TestMigration014(t *testing.T) {
	// Execute Migration Up
	migration := migrations.Migration014{}
	if err := migration.Up("", "", true); err == nil {
		t.Fatal("expected deprecation failure, but was nil")
	}
	// Execute Migration Down
	if err := migration.Down("", "", true); err == nil {
		t.Fatal("expected deprecation failure, but was nil")
	}
}
