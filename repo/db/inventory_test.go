package db

import (
	"database/sql"
	"strconv"
	"sync"
	"testing"
)

var ivdb InventoryDB

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	ivdb = InventoryDB{
		db:   conn,
		lock: new(sync.Mutex),
	}
}

func TestPutInventory(t *testing.T) {
	err := ivdb.Put("abc", 5)
	if err != nil {
		t.Error(err)
	}
	stmt, err := ivdb.db.Prepare("select slug, count from inventory where slug=?")
	defer stmt.Close()
	var slug string
	var count int
	err = stmt.QueryRow("abc").Scan(&slug, &count)
	if err != nil {
		t.Error(err)
	}
	if slug != "abc" {
		t.Errorf(`Expected "abc" got %s`, slug)
	}
	if count != 5 {
		t.Errorf(`Expected 5 got %d`, count)
	}
}

func TestPutReplaceInventory(t *testing.T) {
	ivdb.Put("abc", 6)
	err := ivdb.Put("abc", 5)
	if err != nil {
		t.Error("Error replacing inventory value")
	}
}

func TestGetInventory(t *testing.T) {
	ivdb.Put("abc", 5)
	count, err := ivdb.Get("abc")
	if err != nil || count != 5 {
		t.Error("Error in inventory get")
	}
	count, err = ivdb.Get("xyz")
	if err == nil {
		t.Error("Error in inventory get")
	}
}

func TestDeleteInventory(t *testing.T) {
	ivdb.Put("abc", 5)
	err := ivdb.Delete("abc")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := ivdb.db.Prepare("select slug from inventory where slug=?")
	defer stmt.Close()
	var slug string
	stmt.QueryRow("abc").Scan(&slug)
	if slug != "" {
		t.Error("Failed to delete follower")
	}
}

func TestGetAllInventory(t *testing.T) {
	for i := 0; i < 100; i++ {
		ivdb.Put(strconv.Itoa(i), i)
	}
	inventory, err := ivdb.GetAll()
	if err != nil {
		t.Error(err)
	}
	if len(inventory) != 100 {
		t.Error("Failed to get all inventory")
	}
}
