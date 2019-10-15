package db_test

import (
	"math/big"
	"sync"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func buildNewInventoryStore() (repo.InventoryStore, func(), error) {
	appSchema := schema.MustNewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err := appSchema.BuildSchemaDirectories(); err != nil {
		return nil, nil, err
	}
	if err := appSchema.InitializeDatabase(); err != nil {
		return nil, nil, err
	}
	database, err := appSchema.OpenDatabase()
	if err != nil {
		return nil, nil, err
	}
	return db.NewInventoryStore(database, new(sync.Mutex)), appSchema.DestroySchemaDirectories, nil
}

func TestPutInventory(t *testing.T) {
	ivdb, teardown, err := buildNewInventoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = ivdb.Put("slug", 0, big.NewInt(5))
	if err != nil {
		t.Error(err)
	}
	stmt, err := ivdb.PrepareQuery("select slug, variantIndex, count from inventory where slug=?")
	if err != nil {
		t.Error(err)
	}
	defer stmt.Close()
	var slug string
	var variant int
	var count int
	err = stmt.QueryRow("slug").Scan(&slug, &variant, &count)
	if err != nil {
		t.Error(err)
	}
	if slug != "slug" {
		t.Errorf(`Expected "slug" got %s`, slug)
	}
	if variant != 0 {
		t.Errorf(`Expected 0 got %d`, variant)
	}
	if count != 5 {
		t.Errorf(`Expected 5 got %d`, count)
	}
}

func TestPutReplaceInventory(t *testing.T) {
	ivdb, teardown, err := buildNewInventoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = ivdb.Put("slug", 0, big.NewInt(6))
	if err != nil {
		t.Log(err)
	}
	err = ivdb.Put("slug", 0, big.NewInt(5))
	if err != nil {
		t.Error("Error replacing inventory value")
	}
}

func TestGetSpecificInventory(t *testing.T) {
	ivdb, teardown, err := buildNewInventoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = ivdb.Put("slug", 0, big.NewInt(5))
	if err != nil {
		t.Log(err)
	}
	count, err := ivdb.GetSpecific("slug", 0)
	if err != nil || count.Cmp(big.NewInt(5)) != 0 {
		t.Error("Error in inventory get")
	}
	_, err = ivdb.GetSpecific("xyz", 0)
	if err == nil {
		t.Error("Error in inventory get")
	}
}

func TestDeleteInventory(t *testing.T) {
	ivdb, teardown, err := buildNewInventoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = ivdb.Put("slug", 0, big.NewInt(5))
	if err != nil {
		t.Log(err)
	}
	err = ivdb.Delete("slug", 0)
	if err != nil {
		t.Error(err)
	}
	stmt, _ := ivdb.PrepareQuery("select slug from inventory where slug=?")
	defer stmt.Close()
	var slug string
	err = stmt.QueryRow("inventory").Scan(&slug)
	if err != nil {
		t.Log(err)
	}
	if slug != "" {
		t.Error("Failed to delete inventory")
	}
}

func TestDeleteAllInventory(t *testing.T) {
	ivdb, teardown, err := buildNewInventoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = ivdb.Put("slug", 0, big.NewInt(5))
	if err != nil {
		t.Log(err)
	}
	err = ivdb.Put("slug", 1, big.NewInt(10))
	if err != nil {
		t.Log(err)
	}
	err = ivdb.DeleteAll("slug")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := ivdb.PrepareQuery("select slug from inventory where slug=?")
	defer stmt.Close()
	var slug string
	err = stmt.QueryRow("slug").Scan(&slug)
	if err != nil {
		t.Log(err)
	}
	if slug != "" {
		t.Error("Failed to delete inventory")
	}
}

func TestGetAllInventory(t *testing.T) {
	ivdb, teardown, err := buildNewInventoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	for i := 0; i < 100; i++ {
		err = ivdb.Put("slug1", i, big.NewInt(int64(i)))
		if err != nil {
			t.Log(err)
		}
	}
	for i := 0; i < 100; i++ {
		err = ivdb.Put("slug2", i, big.NewInt(int64(i)))
		if err != nil {
			t.Log(err)
		}
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
	ivdb, teardown, err := buildNewInventoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	for i := 0; i < 100; i++ {
		err = ivdb.Put("slug", i, big.NewInt(int64(i)))
		if err != nil {
			t.Log(err)
		}
	}
	inventory, err := ivdb.Get("slug")
	if err != nil {
		t.Error(err)
	}
	if len(inventory) != 100 {
		t.Error("Failed to get all inventory")
	}
}
