package migrations_test

import (
	"database/sql"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
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
					paymentCoin not null default '');`

func TestMigration026(t *testing.T) {
	var dbPath string
	os.Mkdir("./datastore", os.ModePerm)
	dbPath = path.Join("./", "datastore", "mainnet.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Error(err)
	}
	db.Exec(stmt)
	_, err = db.Exec("INSERT INTO sales (orderID, total) values (?,?)", "asdf", 3)
	if err != nil {
		t.Error(err)
		return
	}
	_, err = db.Exec("INSERT INTO purchases (orderID, total) values (?,?)", "asdf", 3)
	if err != nil {
		t.Error(err)
		return
	}
	var m migrations.Migration026
	err = m.Up("./", "letmein", false)
	if err != nil {
		t.Error(err)
	}

	var orderID string
	var total string
	var total1 int

	r := db.QueryRow("select orderID, total from sales where orderID=?", "asdf")

	if err := r.Scan(&orderID, &total); err != nil || total != "3" {
		t.Error(err)
		return
	}

	r = db.QueryRow("select orderID, total from purchases where orderID=?", "asdf")

	if err := r.Scan(&orderID, &total); err != nil || total != "3" {
		t.Error(err)
		return
	}

	repoVer, err := ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "27" {
		t.Error("Failed to write new repo version")
	}

	err = m.Down("./", "letmein", false)
	if err != nil {
		t.Error(err)
		return
	}

	r = db.QueryRow("select orderID, total from sales where orderID=?", "asdf")

	if err := r.Scan(&orderID, &total1); err != nil || total1 != 3 {
		t.Error(err)
		return
	}

	r = db.QueryRow("select orderID, total from purchases where orderID=?", "asdf")

	if err := r.Scan(&orderID, &total1); err != nil || total1 != 3 {
		t.Error(err)
		return
	}

	repoVer, err = ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "26" {
		t.Error("Failed to write new repo version")
	}
	os.RemoveAll("./datastore")
	os.RemoveAll("./repover")
}
