package db

import (
	"database/sql"
	"encoding/json"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"testing"
	"sync"
)

var sdb SettingsDB
var settings repo.SettingsData

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	sdb = SettingsDB{
		db: conn,
		lock: new(sync.Mutex),
	}
	c := "UNITED_STATES"
	settings = repo.SettingsData{
		Country: &c,
	}
}

func TestSettingsPut(t *testing.T) {
	err := sdb.Put(settings)
	if err != nil {
		t.Error(err)
	}
	set := repo.SettingsData{}
	stmt, err := sdb.db.Prepare("select value from config where key=?")
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
	tx, err := sdb.db.Begin()
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
	err := sdb.Put(settings)
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
	err := sdb.Put(settings)
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
