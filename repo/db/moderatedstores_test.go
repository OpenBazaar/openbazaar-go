package db_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func buildNewModeratedStore() (repo.ModeratedStore, func(), error) {
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
	return db.NewModeratedStore(database, new(sync.Mutex)), appSchema.DestroySchemaDirectories, nil
}

func TestModeratedDB_Put(t *testing.T) {
	modDB, teardown, err := buildNewModeratedStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = modDB.Put("abc")
	if err != nil {
		t.Error(err)
	}
	stmt, err := modDB.PrepareQuery("select peerID from moderatedstores where peerID=?")
	if err != nil {
		t.Error(err)
	}
	defer stmt.Close()
	var peerId string
	err = stmt.QueryRow("abc").Scan(&peerId)
	if err != nil {
		t.Error(err)
	}
	if peerId != "abc" {
		t.Errorf(`Expected "abc" got %s`, peerId)
	}
}

func TestModeratedDB_Put_Duplicate(t *testing.T) {
	modDB, teardown, err := buildNewModeratedStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	modDB.Put("abc")
	err = modDB.Put("abc")
	if err == nil {
		t.Error("Expected unquire constriant error to be thrown")
	}
}

func TestModeratedDB_Delete(t *testing.T) {
	modDB, teardown, err := buildNewModeratedStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	modDB.Put("abc")
	err = modDB.Delete("abc")
	if err != nil {
		t.Error(err)
	}
	stmt, _ := modDB.PrepareQuery("select peerID from moderatedstores where peerID=?")
	defer stmt.Close()
	var peerId string
	stmt.QueryRow("abc").Scan(&peerId)
	if peerId != "" {
		t.Error("Failed to delete moderated store")
	}
}

func TestModeratedDB_Get(t *testing.T) {
	modDB, teardown, err := buildNewModeratedStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	for i := 0; i < 100; i++ {
		modDB.Put(strconv.Itoa(i))
	}
	stores, err := modDB.Get("", 100)
	if err != nil {
		t.Error(err)
	}
	for i := 0; i < 100; i++ {
		f, _ := strconv.Atoi(stores[i])
		if f != 99-i {
			t.Errorf("Returned %d expected %d", f, 99-i)
		}
	}

	stores, err = modDB.Get(strconv.Itoa(30), 100)
	if err != nil {
		t.Error(err)
	}
	for i := 0; i < 30; i++ {
		f, _ := strconv.Atoi(stores[i])
		if f != 29-i {
			t.Errorf("Returned %d expected %d", f, 29-i)
		}
	}
	if len(stores) != 30 {
		t.Error("Incorrect number of moderated stores returned")
	}

	stores, err = modDB.Get(strconv.Itoa(30), 5)
	if err != nil {
		t.Error(err)
	}
	if len(stores) != 5 {
		t.Error("Incorrect number of moderated stores returned")
	}
	for i := 0; i < 5; i++ {
		f, _ := strconv.Atoi(stores[i])
		if f != 29-i {
			t.Errorf("Returned %d expected %d", f, 29-i)
		}
	}
}
