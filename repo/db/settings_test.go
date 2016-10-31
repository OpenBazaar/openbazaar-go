package db

import (
	"database/sql"
	"encoding/json"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"sync"
	"testing"
)

var sdb SettingsDB
var settings repo.SettingsData

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	sdb = SettingsDB{
		db:   conn,
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
		t.Error("settings get did not error with invalid data")
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
	l := "English"
	setUpdt := repo.SettingsData{
		Language: &l,
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
	if *set.Language != "English" {
		t.Error("Settings update failed to put correct value")
	}
	if *set.TermsAndConditions != "None" {
		t.Error("Settings update failed to put correct value")
	}
}
