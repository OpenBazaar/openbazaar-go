package migrations_test

import (
	"bytes"
	"database/sql"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/wallet-interface"
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
