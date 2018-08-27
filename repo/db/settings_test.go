package db_test

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func buildConfigurationStore() (repo.ConfigurationStore, func(), error) {
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
	return db.NewConfigurationStore(database, new(sync.Mutex)), appSchema.DestroySchemaDirectories, nil
}

func TestSettingsPut(t *testing.T) {
	sdb, teardown, err := buildConfigurationStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	country := "UNITED_STATES"
	settings := repo.SettingsData{
		Country: &country,
	}
	err = sdb.Put(settings)
	if err != nil {
		t.Error(err)
	}
	set := repo.SettingsData{}
	stmt, err := sdb.PrepareQuery("select value from config where key=?")
	if err != nil {
		t.Error(err)
	}
	defer stmt.Close()
	var settingsBytes []byte
	err = stmt.QueryRow("settings").Scan(&settingsBytes)
	if err != nil {
		t.Error(err)
	}
	err = json.Unmarshal(settingsBytes, &set)
	if err != nil {
		t.Error(err)
	}
	if *set.Country != "UNITED_STATES" {
		t.Error("Settings put failed to put correct value")
	}
}

func TestInvalidSettingsGet(t *testing.T) {
	sdb, teardown, err := buildConfigurationStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	tx, err := sdb.BeginTransaction()
	if err != nil {
		t.Error(err)
	}
	stmt, _ := tx.Prepare("insert or replace into config(key, value) values(?,?)")
	defer stmt.Close()

	_, err = stmt.Exec("settings", string("Test fail"))
	if err != nil {
		tx.Rollback()
		t.Error(err)
	}
	tx.Commit()
	_, err = sdb.Get()
	if err == nil {
		t.Error("settings get didn't error with invalid data")
	}
}

func TestSettingsGet(t *testing.T) {
	sdb, teardown, err := buildConfigurationStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	country := "UNITED_STATES"
	settings := repo.SettingsData{
		Country: &country,
	}
	err = sdb.Put(settings)
	if err != nil {
		t.Error(err)
	}
	set, err := sdb.Get()
	if err != nil {
		t.Error(err)
	}
	if *set.Country != "UNITED_STATES" {
		t.Error("Settings put failed to put correct value")
	}
}

func TestSettingsUpdate(t *testing.T) {
	sdb, teardown, err := buildConfigurationStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	country := "UNITED_STATES"
	settings := repo.SettingsData{
		Country: &country,
	}
	err = sdb.Put(settings)
	if err != nil {
		t.Error(err)
	}
	l := "/openbazaar-go:0.4/"
	setUpdt := repo.SettingsData{
		Version: &l,
	}
	err = sdb.Update(setUpdt)
	if err != nil {
		t.Error(err)
	}
	r := "None"
	setUpdt2 := repo.SettingsData{
		TermsAndConditions: &r,
	}
	err = sdb.Update(setUpdt2)
	if err != nil {
		t.Error(err)
	}
	set, err := sdb.Get()
	if err != nil {
		t.Error(err)
	}
	if *set.Country != "UNITED_STATES" {
		t.Error("Settings update failed to put correct value")
	}
	if *set.Version != "/openbazaar-go:0.4/" {
		t.Error("Settings update failed to put correct value")
	}
	if *set.TermsAndConditions != "None" {
		t.Error("Settings update failed to put correct value")
	}
}
