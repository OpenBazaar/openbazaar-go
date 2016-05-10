package db

import (
	"path"
	"sync"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("db")

type SQLiteDatastore struct {
	followers repo.Followers
	db *sql.DB
}

func Create(repoPath string) (*SQLiteDatastore, error) {
	dbPath := path.Join(repoPath, "datastore", "mainnet.db")
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	l := new(sync.Mutex)
	sqliteDB := &SQLiteDatastore{
		followers: &FollowerDB{
			db: conn,
			lock: l,
		},
	}

	return sqliteDB, nil
}

func (d *SQLiteDatastore) Close() {
	d.db.Close()
}

func (d *SQLiteDatastore) Followers() repo.Followers {
	return d.followers
}
