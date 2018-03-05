package core

import (
	"database/sql"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
)

func TestDisputeNotifierUpdatesLastNotifiedAt(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	setupSQL := []string{
		db.PragmaKey(""),
		db.CreateTableDisputedCasesSQL,
	}
	_, err := database.Exec(strings.Join(setupSQL, " "))
	if err != nil {
		t.Fatal(err)
	}
	timeStart := time.Now().Add(time.Duration(-5) * time.Minute)
	existingRecords := []*repo.DisputeCaseRecord{
		{
			CaseID:         "test1",
			Timestamp:      timeStart.Unix(),
			LastNotifiedAt: 0,
		},
		{
			CaseID:         "test2",
			Timestamp:      timeStart.Unix(),
			LastNotifiedAt: timeStart.Unix(),
		},
	}

	for _, r := range existingRecords {
		_, err := database.Exec("insert into cases (caseID, timestamp, lastNotifiedAt) values (?, ?, ?);", r.CaseID, r.Timestamp, r.LastNotifiedAt)
		if err != nil {
			t.Fatal(err)
		}
	}

	worker := &disputeNotifier{
		disputeCasesDb: db.NewCaseStore(database, new(sync.Mutex)),
	}
	if err := worker.PerformTask(); err != nil {
		t.Fatal(err)
	}

	rows, err := database.Query("select caseID, lastNotifiedAt from cases;")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var r repo.DisputeCaseRecord
		if err := rows.Scan(&r.CaseID, &r.LastNotifiedAt); err != nil {
			t.Fatal(err)
		}
		actualTime := time.Unix(r.LastNotifiedAt, 0)
		durationFromActual := time.Now().Sub(actualTime)
		if durationFromActual > (time.Duration(5) * time.Second) {
			t.Errorf("Expected %s to have lastNotifiedAt set when executed, was %s", r.CaseID, actualTime.String())
		}
	}
}
