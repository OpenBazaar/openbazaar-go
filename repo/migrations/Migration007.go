package migrations

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/util"
)

const migration007_casesCreateSQL = "CREATE TABLE cases (caseID text primary key not null, buyerContract blob, vendorContract blob, buyerValidationErrors blob, vendorValidationErrors blob, buyerPayoutAddress text, vendorPayoutAddress text, buyerOutpoints blob, vendorOutpoints blob, state integer, read integer, timestamp integer, buyerOpened integer, claim text, disputeResolution blob);"

type Migration007 struct{}

func (Migration007) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	paths, err := util.NewCustomSchemaManager(util.SchemaContext{
		DataPath:        repoPath,
		TestModeEnabled: testnetEnabled,
	})
	if err != nil {
		return err
	}

	// Add lastNotifiedAt column
	db, err := sql.Open("sqlite3", paths.DatastorePath())
	if err != nil {
		return err
	}
	defer db.Close()
	if databasePassword != "" {
		p := fmt.Sprintf("pragma key = '%s';", databasePassword)
		_, err := db.Exec(p)
		if err != nil {
			return err
		}
	}
	_, err = db.Exec("alter table cases add column lastNotifiedAt integer not null default 0")
	if err != nil {
		return err
	}

	_, err = db.Exec("update cases set lastNotifiedAt = ?", time.Now().Unix())
	if err != nil {
		return err
	}

	// Bump schema version
	err = ioutil.WriteFile(paths.DataPathJoin("repover"), []byte("8"), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func (Migration007) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	paths, err := util.NewCustomSchemaManager(util.SchemaContext{
		DataPath:        repoPath,
		TestModeEnabled: testnetEnabled,
	})
	if err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", paths.DatastorePath())
	if err != nil {
		return err
	}
	defer db.Close()
	if databasePassword != "" {
		p := fmt.Sprintf("pragma key = '%s';", databasePassword)
		_, err := db.Exec(p)
		if err != nil {
			return err
		}
	}

	const (
		removeColumnSQL = "alter table cases rename to cases_old;"
		insertCasesSQL  = "insert into cases select caseID, buyerContract, vendorContract, buyerValidationErrors, vendorValidationErrors, buyerPayoutAddress, vendorPayoutAddress, buyerOutpoints, vendorOutpoints, state, read, timestamp, buyerOpened, claim, disputeResolution from cases_old;"
		dropTableSQL    = "drop table cases_old;"
	)

	dropColumnOperation := strings.Join([]string{
		removeColumnSQL,
		migration007_casesCreateSQL,
		insertCasesSQL,
		dropTableSQL,
	}, " ")
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(dropColumnOperation); err != nil {
		tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	// Revert schema version
	err = ioutil.WriteFile(paths.DataPathJoin("repover"), []byte("7"), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}
