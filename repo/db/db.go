package db

import (
	"path"
	"sync"
	"database/sql"
	_ "github.com/xeodou/go-sqlcipher"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("db")

type SQLiteDatastore struct {
	config    repo.Config
	followers repo.Followers
	db        *sql.DB
	lock      *sync.Mutex
}

func Create(repoPath, password string, testnet bool) (*SQLiteDatastore, error) {
	var dbPath string
	if testnet {
		dbPath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		dbPath = path.Join(repoPath, "datastore", "mainnet.db")
	}
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	if password != "" {
		p := "pragma key = '" + password + "';"
		conn.Exec(p)
	}

	l := new(sync.Mutex)
	sqliteDB := &SQLiteDatastore{
		config: &ConfigDB{
			db: conn,
			lock: l,
			path: dbPath,
		},
		followers: &FollowerDB{
			db: conn,
			lock: l,
		},
		db: conn,
		lock: l,
	}

	return sqliteDB, nil
}

func (d *SQLiteDatastore) Close() {
	d.db.Close()
}

func (d *SQLiteDatastore) Config() repo.Config {
	return d.config
}

func (d *SQLiteDatastore) Followers() repo.Followers {
	return d.followers
}

func (d *SQLiteDatastore) Copy(dbPath string, password string) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	var cp string
	if password == "" {
		cp = `
			attach database '` + dbPath + `' as plaintext key '';
			insert into plaintext.followers select * from main.followers
		`
	} else {
		cp = `
			attach database '` + dbPath + `' as encrypted key '`+ password +`';
			insert into encrypted.followers select * from main.followers
		`
	}

	_, err := d.db.Exec(cp)
	if err != nil {
		return err
	}

	return nil
}

func initDatabaseTables(db *sql.DB, password string) error {
	var sqlStmt string
	if password != "" {
		sqlStmt = "PRAGMA key = '" + password + "';"
	}
	sqlStmt = sqlStmt + `
	create table followers (peerID text primary key not null);
	create table config (key text primary key not null, value blob);
	`
	_, err := db.Exec(sqlStmt)
	if err != nil {
		return err
	}
	return nil
}

type ConfigDB struct {
	db   *sql.DB
	lock *sync.Mutex
	path string
}

func (c *ConfigDB) Init(mnemonic string, identityKey []byte, password string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := initDatabaseTables(c.db, password); err != nil {
		return err
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert into config(key, value) values(?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec("mnemonic", mnemonic)
	if err != nil {
		return err
	}
	_, err = stmt.Exec("identityKey", identityKey)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil

}

func (c *ConfigDB) GetMnemonic() (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	stmt, err := c.db.Prepare("select value from config where key=?")
	defer stmt.Close()
	var mnemonic string
	err = stmt.QueryRow("mnemonic").Scan(&mnemonic)
	if err != nil {
		log.Fatal(err)
	}
	return mnemonic, nil
}

func (c *ConfigDB) GetIdentityKey() ([]byte, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	stmt, err := c.db.Prepare("select value from config where key=?")
	defer stmt.Close()
	var identityKey []byte
	err = stmt.QueryRow("identityKey").Scan(&identityKey)
	if err != nil {
		log.Fatal(err)
	}
	return identityKey, nil
}

func (c *ConfigDB) IsEncrypted() bool {
	pwdCheck := "select count(*) from sqlite_master;"
	_, err := c.db.Exec(pwdCheck) // fails is wrong pw entered
	if err != nil {
		return true
	}
	return false
}


