package db

import (
	"database/sql"
	"path"
	"sync"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/spvwallet"
	"github.com/mattes/migrate"
	_ "github.com/mattes/migrate/source/file"
	"github.com/op/go-logging"
	"time"
)

var log = logging.MustGetLogger("db")

type SQLiteDatastore struct {
	config          repo.Config
	followers       repo.Followers
	following       repo.Following
	offlineMessages repo.OfflineMessages
	pointers        repo.Pointers
	keys            spvwallet.Keys
	stxos           spvwallet.Stxos
	txns            spvwallet.Txns
	utxos           spvwallet.Utxos
	watchedScripts  spvwallet.WatchedScripts
	settings        repo.Settings
	inventory       repo.Inventory
	purchases       repo.Purchases
	sales           repo.Sales
	cases           repo.Cases
	chat            repo.Chat
	notifications   repo.Notifications
	coupons         repo.Coupons
	txMetadata      repo.TxMetadata
	moderatedStores repo.ModeratedStores
	db              *sql.DB
	lock            sync.RWMutex
}

func Create(repoPath, password string, testnet bool, migrationsPath string) (*SQLiteDatastore, error) {
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
		p := "pragma key='" + password + "';"
		conn.Exec(p)
	}
	var l sync.RWMutex
	sqliteDB := &SQLiteDatastore{
		config: &ConfigDB{
			db:   conn,
			lock: l,
			path: dbPath,
		},
		followers: &FollowerDB{
			db:   conn,
			lock: l,
		},
		following: &FollowingDB{
			db:   conn,
			lock: l,
		},
		offlineMessages: &OfflineMessagesDB{
			db:   conn,
			lock: l,
		},
		pointers: &PointersDB{
			db:   conn,
			lock: l,
		},
		keys: &KeysDB{
			db:   conn,
			lock: l,
		},
		stxos: &StxoDB{
			db:   conn,
			lock: l,
		},
		txns: &TxnsDB{
			db:   conn,
			lock: l,
		},
		utxos: &UtxoDB{
			db:   conn,
			lock: l,
		},
		settings: &SettingsDB{
			db:   conn,
			lock: l,
		},
		inventory: &InventoryDB{
			db:   conn,
			lock: l,
		},
		purchases: &PurchasesDB{
			db:   conn,
			lock: l,
		},
		sales: &SalesDB{
			db:   conn,
			lock: l,
		},
		watchedScripts: &WatchedScriptsDB{
			db:   conn,
			lock: l,
		},
		cases: &CasesDB{
			db:   conn,
			lock: l,
		},
		chat: &ChatDB{
			db:   conn,
			lock: l,
		},
		notifications: &NotficationsDB{
			db:   conn,
			lock: l,
		},
		coupons: &CouponDB{
			db:   conn,
			lock: l,
		},
		txMetadata: &TxMetadataDB{
			db:   conn,
			lock: l,
		},
		moderatedStores: &ModeratedDB{
			db:   conn,
			lock: l,
		},
		db:   conn,
		lock: l,
	}

	// Migrations
	if err = initDatabase(conn, migrationsPath); err != nil {
		return nil, err
	}
	return sqliteDB, nil
}

func initDatabase(db *sql.DB, migrationsPath string) error {
	driver, err := WithInstance(db, &Config{
		MigrationsTable: DefaultMigrationsTable,
		DatabaseName:    "sqlite3",
	})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"sqlite3", driver)
	if err != nil {
		log.Error(err)
		return err
	}
	err = m.Up()
	if err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func (d *SQLiteDatastore) Ping() error {
	return d.db.Ping()
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

func (d *SQLiteDatastore) Following() repo.Following {
	return d.following
}

func (d *SQLiteDatastore) OfflineMessages() repo.OfflineMessages {
	return d.offlineMessages
}

func (d *SQLiteDatastore) Pointers() repo.Pointers {
	return d.pointers
}

func (d *SQLiteDatastore) Keys() spvwallet.Keys {
	return d.keys
}

func (d *SQLiteDatastore) Stxos() spvwallet.Stxos {
	return d.stxos
}

func (d *SQLiteDatastore) Txns() spvwallet.Txns {
	return d.txns
}

func (d *SQLiteDatastore) Utxos() spvwallet.Utxos {
	return d.utxos
}

func (d *SQLiteDatastore) Settings() repo.Settings {
	return d.settings
}

func (d *SQLiteDatastore) Inventory() repo.Inventory {
	return d.inventory
}

func (d *SQLiteDatastore) Purchases() repo.Purchases {
	return d.purchases
}

func (d *SQLiteDatastore) Sales() repo.Sales {
	return d.sales
}

func (d *SQLiteDatastore) WatchedScripts() spvwallet.WatchedScripts {
	return d.watchedScripts
}

func (d *SQLiteDatastore) Cases() repo.Cases {
	return d.cases
}

func (d *SQLiteDatastore) Chat() repo.Chat {
	return d.chat
}

func (d *SQLiteDatastore) Notifications() repo.Notifications {
	return d.notifications
}

func (d *SQLiteDatastore) Coupons() repo.Coupons {
	return d.coupons
}

func (d *SQLiteDatastore) TxMetadata() repo.TxMetadata {
	return d.txMetadata
}

func (d *SQLiteDatastore) ModeratedStores() repo.ModeratedStores {
	return d.moderatedStores
}

func (d *SQLiteDatastore) Copy(dbPath string, password string) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	var cp string
	stmt := "select name from sqlite_master where type='table'"
	rows, err := d.db.Query(stmt)
	if err != nil {
		log.Error(err)
		return err
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		tables = append(tables, name)
	}
	if password == "" {
		cp = `attach database '` + dbPath + `' as plaintext key '';`
		for _, name := range tables {
			if name != "schema_migrations" {
				cp = cp + "insert into plaintext." + name + " select * from main." + name + ";"
			}
		}
	} else {
		cp = `attach database '` + dbPath + `' as encrypted key '` + password + `';`
		for _, name := range tables {
			if name != "schema_migrations" {
				cp = cp + "insert into encrypted." + name + " select * from main." + name + ";"
			}
		}
	}

	_, err = d.db.Exec(cp)
	if err != nil {
		return err
	}

	return nil
}

func decryptDatabase(db *sql.DB, password string) error {
	if password != "" {
		sqlStmt := "PRAGMA key = '" + password + "';"
		_, err := db.Exec(sqlStmt)
		if err != nil {
			return err
		}
	}
	return nil
}

type ConfigDB struct {
	db   *sql.DB
	lock sync.RWMutex
	path string
}

func (c *ConfigDB) Init(mnemonic string, identityKey []byte, password string, creationDate time.Time) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := decryptDatabase(c.db, password); err != nil {
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
		tx.Rollback()
		return err
	}
	_, err = stmt.Exec("identityKey", identityKey)
	if err != nil {
		tx.Rollback()
		return err
	}
	_, err = stmt.Exec("creationDate", creationDate.Format(time.RFC3339))
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (c *ConfigDB) GetMnemonic() (string, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
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
	c.lock.RLock()
	defer c.lock.RUnlock()
	stmt, err := c.db.Prepare("select value from config where key=?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	var identityKey []byte
	err = stmt.QueryRow("identityKey").Scan(&identityKey)
	if err != nil {
		return nil, err
	}
	return identityKey, nil
}

func (c *ConfigDB) GetCreationDate() (time.Time, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	var t time.Time
	stmt, err := c.db.Prepare("select value from config where key=?")
	if err != nil {
		return t, err
	}
	defer stmt.Close()
	var creationDate []byte
	err = stmt.QueryRow("creationDate").Scan(&creationDate)
	if err != nil {
		return t, err
	}
	return time.Parse(time.RFC3339, string(creationDate))
}

func (c *ConfigDB) IsEncrypted() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	pwdCheck := "select count(*) from sqlite_master;"
	_, err := c.db.Exec(pwdCheck) // Fails if wrong password is entered
	if err != nil {
		return true
	}
	return false
}
