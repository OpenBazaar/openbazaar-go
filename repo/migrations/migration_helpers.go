package migrations

import (
	"database/sql"
	"os"
	"path"
	"strconv"
)

func NewDB(repoPath string, dbPassword string, testnet bool) (*sql.DB, error) {
	var dbPath string
	if testnet {
		dbPath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		dbPath = path.Join(repoPath, "datastore", "mainnet.db")
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	if dbPassword != "" {
		p := "pragma key='" + dbPassword + "';"
		_, err = db.Exec(p)
		if err != nil {
			return nil, err
		}
	}
	return db, nil
}

func withTransaction(db *sql.DB, handler func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	err = handler(tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func writeRepoVer(repoPath string, version int) error {
	f1, err := os.Create(path.Join(repoPath, "repover"))
	if err != nil {
		return err
	}
	_, err = f1.Write([]byte(strconv.Itoa(version)))
	if err != nil {
		return err
	}
	return f1.Close()
}
