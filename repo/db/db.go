package db

import (
	"path"
	"sync"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

type SQLiteDatastore struct {
	followers repo.Followers

	db *sql.DB
	lock sync.Mutex
}

func Create(repoPath string) (*SQLiteDatastore, error) {
	dbPath := path.Join(repoPath, "datastore", "mainnet.db")
	dbConn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	sqliteDB := &SQLiteDatastore{
		followers: &FollowerDB{
			db: dbConn,
		},
		db: dbConn,

	}
	return sqliteDB, nil
}

func (d *SQLiteDatastore) Followers() repo.Followers {
	return d.followers
}