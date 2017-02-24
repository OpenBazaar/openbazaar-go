package db

import (
	"database/sql"
	"strconv"
	"testing"
)

var ivdb InventoryDB

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	ivdb = InventoryDB{
		db: conn,
	}
}

func TestPutInventory(t *testing.T) {
	err := ivdb.Put("slug", "[0, 1]", 5)
	if err != nil {
		t.Error(err)
	}
	stmt, err := ivdb.db.Prepare("select slug, variant, count from inventory where slug=?")
	defer stmt.Close()
	var slug string
	var variant string
	var count int
	err = stmt.QueryRow("slug").Scan(&slug, &variant, &count)
	if err != nil {
		t.Error(err)
	}
	if slug != "slug" {
		t.Errorf(`Expected "slug" got %s`, slug)
	}
	if variant != "[0, 1]" {
		t.Errorf(`Expected [0, 1] got %s`, variant)
	}
	if count != 5 {
		t.Errorf(`Expected 5 got %d`, count)
	}
}

func TestPutReplaceInventory(t *testing.T) {
	ivdb.Put("slug", "[0, 1]", 6)
	err := ivdb.Put("slug", "[0, 1]", 5)
	if err != nil {
		t.Error("Error replacing inventory value")
	}
}

func TestGetSpecificInventory(t *testing.T) {
	ivdb.Put("slug", "[0, 1]", 5)
	count, err := ivdb.GetSpecific("slug", "[0, 1]")
	if err != nil || count != 5 {
		t.Error("Error in inventory get")
	}
	count, err = ivdb.GetSpecific("xyz", "[0, 1]")
	if err == nil {
		t.Error("Error in inventory get")
	}
}

func TestDeleteInventory(t *testing.T) {
	ivdb.Put("slug", "[0, 1]", 5)
	err := ivdb.Delete("slug", "[0, 1]")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := ivdb.db.Prepare("select variant from inventory where slug=?")
	defer stmt.Close()
	var variant string
	stmt.QueryRow("inventory").Scan(&variant)
	if variant != "" {
		t.Error("Failed to delete inventory")
	}
}

func TestDeleteAllInventory(t *testing.T) {
	ivdb.Put("slug", "[0, 1]", 5)
	ivdb.Put("slug", "[0, 2]", 10)
	err := ivdb.DeleteAll("slug")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := ivdb.db.Prepare("select variant from inventory where slug=?")
	defer stmt.Close()
	var variant string
	stmt.QueryRow("slug").Scan(&variant)
	if variant != "" {
		t.Error("Failed to delete inventory")
	}
}

func TestGetAllInventory(t *testing.T) {
	for i := 0; i < 100; i++ {
		ivdb.Put("slug1", strconv.Itoa(i), i)
	}
	for i := 0; i < 100; i++ {
		ivdb.Put("slug2", strconv.Itoa(i), i)
	}
	inventory, err := ivdb.GetAll()
	if err != nil {
		t.Error(err)
	}
	if len(inventory) != 2 {
		t.Error("Failed to get all inventory")
	}
	if len(inventory["slug1"]) != 100 {
		t.Error("Failed to get all inventory")
	}
	if len(inventory["slug2"]) != 100 {
		t.Error("Failed to get all inventory")
	}
}

func TestGetInventory(t *testing.T) {
	for i := 0; i < 100; i++ {
		ivdb.Put("slug", strconv.Itoa(i), i)
	}
	inventory, err := ivdb.Get("slug")
	if err != nil {
		t.Error(err)
	}
	if len(inventory) != 100 {
		t.Error("Failed to get all inventory")
	}
}
