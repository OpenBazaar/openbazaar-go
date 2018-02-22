package migrations

import (
	"database/sql"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

var stm = `PRAGMA key = 'letmein';create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob);`

func TestMigration004(t *testing.T) {
	var dbPath string
	os.Mkdir("./datastore", os.ModePerm)
	dbPath = path.Join("./", "datastore", "mainnet.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Error(err)
	}
	db.Exec(stm)
	_, err = db.Exec("INSERT INTO sales (orderID, contract, state, read, timestamp, total, thumbnail, buyerID, buyerHandle, title, shippingName, shippingAddress, paymentAddr, funded, transactions) values (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", "asdf", "{}", 3, 1, 12345, 100000, "zasfd", "Qm...", "name.id", "Listing title", "Peter Griffin", "1234 Quhog st.", "1btc..", 0, []byte("[]"))
	if err != nil {
		t.Error(err)
		return
	}
	var m Migration004
	err = m.Up("./", "letmein", false)
	if err != nil {
		t.Error(err)
	}
	_, err = db.Exec("UPDATE sales set needsSync=? WHERE orderID=?", 1, "asdf")
	if err != nil {
		t.Error(err)
		return
	}
	repoVer, err := ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "5" {
		t.Error("Failed to write new repo version")
	}

	err = m.Down("./", "letmein", false)
	if err != nil {
		t.Error(err)
		return
	}
	_, err = db.Exec("UPDATE sales set needsSync=? WHERE orderID=?", 1, "asdf")
	if err == nil {
		t.Error("Failed to drop columns")
		return
	}
	repoVer, err = ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "4" {
		t.Error("Failed to write new repo version")
	}
	os.RemoveAll("./datastore")
	os.RemoveAll("./repover")
}
