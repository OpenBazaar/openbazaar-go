package migrations_test

import (
	"database/sql"
	"io/ioutil"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

var stmt = `PRAGMA key = 'letmein';
				create table sales (orderID text primary key not null,
					contract blob, state integer, read integer,
					timestamp integer, total integer, thumbnail text,
					buyerID text, buyerHandle text, title text,
					shippingName text, shippingAddress text,
					paymentAddr text, funded integer, transactions blob,
					needsSync integer, lastDisputeTimeoutNotifiedAt integer not null default 0,
					coinType not null default '', paymentCoin not null default '');
				create table purchases (orderID text primary key not null,
					contract blob, state integer, read integer,
					timestamp integer, total integer, thumbnail text,
					vendorID text, vendorHandle text, title text,
					shippingName text, shippingAddress text, paymentAddr text,
					funded integer, transactions blob,
					lastDisputeTimeoutNotifiedAt integer not null default 0,
					lastDisputeExpiryNotifiedAt integer not null default 0,
					disputedAt integer not null default 0, coinType not null default '',
					paymentCoin not null default '');
				create table inventory (invID text primary key not null, slug text, variantIndex integer, count integer);`

func TestMigration029(t *testing.T) {
	basePath := schema.GenerateTempPath()
	appSchema, err := schema.NewCustomSchemaManager(schema.SchemaContext{DataPath: basePath, TestModeEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = appSchema.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer appSchema.DestroySchemaDirectories()

	var dbPath = appSchema.DataPathJoin("datastore", "mainnet.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(stmt); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO sales (orderID, total) values (?,?)", "asdf", 3); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO purchases (orderID, total) values (?,?)", "asdf", 3); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO inventory (invID, count) values (?,?)", "asdf", "3"); err != nil {
		t.Fatal(err)
	}
	var m migrations.Migration029
	if err := m.Up(basePath, "letmein", false); err != nil {
		t.Fatal(err)
	}

	var (
		orderID string
		total   string
		total1  int
		invID   string
		count   string
		count1  int
	)

	r := db.QueryRow("select orderID, total from sales where orderID=?", "asdf")
	if err := r.Scan(&orderID, &total); err != nil {
		t.Error(err)
	}
	if total != "3" {
		t.Errorf("expected total to be 3, but was %s", total)
	}
	r = db.QueryRow("select orderID, total from purchases where orderID=?", "asdf")
	if err := r.Scan(&orderID, &total); err != nil {
		t.Error(err)
	}
	if total != "3" {
		t.Errorf("expected total to be 3, but was %s", total)
	}
	r = db.QueryRow("select invID, count from inventory where invID=?", "asdf")
	if err := r.Scan(&invID, &count); err != nil {
		t.Error(err)
	}
	if count != "3" {
		t.Errorf("expected count to be 3, but was %s", total)
	}

	repoVer, err := ioutil.ReadFile(appSchema.DataPathJoin("repover"))
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "30" {
		t.Error("Failed to write new repo version")
	}

	err = m.Down(basePath, "letmein", false)
	if err != nil {
		t.Fatal(err)
	}

	r = db.QueryRow("select orderID, total from sales where orderID=?", "asdf")
	if err := r.Scan(&orderID, &total1); err != nil {
		t.Error(err)
	}
	if total1 != 3 {
		t.Errorf("expected total to be 3, but was %d", total1)
	}

	r = db.QueryRow("select orderID, total from purchases where orderID=?", "asdf")
	if err := r.Scan(&orderID, &total1); err != nil {
		t.Error(err)
	}
	if total1 != 3 {
		t.Errorf("expected total to be 3, but was %d", total1)
	}

	r = db.QueryRow("select invID, count from inventory where invID=?", "asdf")
	if err := r.Scan(&invID, &count1); err != nil {
		t.Error(err)
	}
	if count1 != 3 {
		t.Errorf("expected count to be 3, but was %d", total1)
	}

	repoVer, err = ioutil.ReadFile(appSchema.DataPathJoin("repover"))
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "29" {
		t.Error("Failed to write old repo version")
	}
}
