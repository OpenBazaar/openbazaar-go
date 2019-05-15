package migrations_test

import (
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
)

type exampleActualMigration struct{}

func (exampleActualMigration) Up(path, pass string, test bool) error   { return nil }
func (exampleActualMigration) Down(path, pass string, test bool) error { return nil }

func ExampleMigrationNoOp() {
	var (
		// migrations 001 and 002 are no-op migrations
		migration001 migrations.MigrationNoOp
		migration002 migrations.MigrationNoOp

		// migration 003 is a literal migration as normal
		migration003 exampleActualMigration
	)

	var _ = []repo.Migration{
		migration001,
		migration002,
		migration003,
	}
}

func TestMigrationNoOpUpNeverFails(t *testing.T) {
	if err := (&migrations.MigrationNoOp{}).Up("", "", true); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationNoOpDownNeverFails(t *testing.T) {
	if err := (&migrations.MigrationNoOp{}).Down("", "", true); err != nil {
		t.Fatal(err)
	}
}
