package db

import (
	"database/sql"
	"path"
	"sync"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/op/go-logging"
	"time"
)

var log = logging.MustGetLogger("db")

type SQLiteDatastore struct {
	config          repo.Config
	followers       repo.FollowerStore
	following       repo.FollowingStore
	offlineMessages repo.OfflineMessageStore
	pointers        repo.PointerStore
	keys            repo.KeyStore
	stxos           repo.SpentTransactionOutputStore
	txns            repo.TransactionStore
	utxos           repo.UnspentTransactionOutputStore
	watchedScripts  repo.WatchedScriptStore
	settings        repo.ConfigurationStore
	inventory       repo.InventoryStore
	purchases       repo.PurchaseStore
	sales           repo.SaleStore
	cases           repo.CaseStore
	chat            repo.ChatStore
	notifications   repo.NotificationStore
	coupons         repo.CouponStore
	txMetadata      repo.TransactionMetadataStore
	moderatedStores repo.ModeratedStore
	db              *sql.DB
	lock            *sync.Mutex
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
		p := "pragma key='" + password + "';"
		conn.Exec(p)
	}
	l := new(sync.Mutex)
	sqliteDB := &SQLiteDatastore{
		config: &ConfigDB{
			db:   conn,
			lock: l,
			path: dbPath,
		},
		followers:       NewFollowerStore(conn, l),
		following:       NewFollowingStore(conn, l),
		offlineMessages: NewOfflineMessageStore(conn, l),
		pointers:        NewPointerStore(conn, l),
		keys:            NewKeyStore(conn, l),
		stxos:           NewSpentTransactionStore(conn, l),
		txns:            NewTransactionStore(conn, l),
		utxos:           NewUnspentTransactionStore(conn, l),
		settings:        NewConfigurationStore(conn, l),
		inventory:       NewInventoryStore(conn, l),
		purchases:       NewPurchaseStore(conn, l),
		sales:           NewSaleStore(conn, l),
		watchedScripts:  NewWatchedScriptStore(conn, l),
		cases:           NewCaseStore(conn, l),
		chat:            NewChatStore(conn, l),
		notifications:   NewNotificationStore(conn, l),
		coupons:         NewCouponStore(conn, l),
		txMetadata:      NewTransactionMetadataStore(conn, l),
		moderatedStores: NewModeratedStore(conn, l),
		db:              conn,
		lock:            l,
	}

	return sqliteDB, nil
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

func (d *SQLiteDatastore) Followers() repo.FollowerStore {
	return d.followers
}

func (d *SQLiteDatastore) Following() repo.FollowingStore {
	return d.following
}

func (d *SQLiteDatastore) OfflineMessages() repo.OfflineMessageStore {
	return d.offlineMessages
}

func (d *SQLiteDatastore) Pointers() repo.PointerStore {
	return d.pointers
}

func (d *SQLiteDatastore) Keys() wallet.Keys {
	return d.keys
}

func (d *SQLiteDatastore) Stxos() wallet.Stxos {
	return d.stxos
}

func (d *SQLiteDatastore) Txns() wallet.Txns {
	return d.txns
}

func (d *SQLiteDatastore) Utxos() wallet.Utxos {
	return d.utxos
}

func (d *SQLiteDatastore) Settings() repo.ConfigurationStore {
	return d.settings
}

func (d *SQLiteDatastore) Inventory() repo.InventoryStore {
	return d.inventory
}

func (d *SQLiteDatastore) Purchases() repo.PurchaseStore {
	return d.purchases
}

func (d *SQLiteDatastore) Sales() repo.SaleStore {
	return d.sales
}

func (d *SQLiteDatastore) WatchedScripts() wallet.WatchedScripts {
	return d.watchedScripts
}

func (d *SQLiteDatastore) Cases() repo.CaseStore {
	return d.cases
}

func (d *SQLiteDatastore) Chat() repo.ChatStore {
	return d.chat
}

func (d *SQLiteDatastore) Notifications() repo.NotificationStore {
	return d.notifications
}

func (d *SQLiteDatastore) Coupons() repo.CouponStore {
	return d.coupons
}

func (d *SQLiteDatastore) TxMetadata() repo.TransactionMetadataStore {
	return d.txMetadata
}

func (d *SQLiteDatastore) ModeratedStores() repo.ModeratedStore {
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
			cp = cp + "insert into plaintext." + name + " select * from main." + name + ";"
		}
	} else {
		cp = `attach database '` + dbPath + `' as encrypted key '` + password + `';`
		for _, name := range tables {
			cp = cp + "insert into encrypted." + name + " select * from main." + name + ";"
		}
	}

	_, err = d.db.Exec(cp)
	if err != nil {
		return err
	}

	return nil
}

func (s *SQLiteDatastore) InitTables(password string) error {
	return initDatabaseTables(s.db, password)
}

func initDatabaseTables(db *sql.DB, password string) error {
	var sqlStmt string
	if password != "" {
		sqlStmt = "PRAGMA key = '" + password + "';"
	}
	sqlStmt += `
	PRAGMA user_version = 0;
	create table config (key text primary key not null, value blob);
	create table followers (peerID text primary key not null, proof blob);
	create table following (peerID text primary key not null);
	create table offlinemessages (url text primary key not null, timestamp integer, message blob);
	create table pointers (pointerID text primary key not null, key text, address text, cancelID text, purpose integer, timestamp integer);
	create table keys (scriptAddress text primary key not null, purpose integer, keyIndex integer, used integer, key text);
	create table utxos (outpoint text primary key not null, value integer, height integer, scriptPubKey text, watchOnly integer);
	create table stxos (outpoint text primary key not null, value integer, height integer, scriptPubKey text, watchOnly integer, spendHeight integer, spendTxid text);
	create table txns (txid text primary key not null, value integer, height integer, timestamp integer, watchOnly integer, tx blob);
	create table txmetadata (txid text primary key not null, address text, memo text, orderID text, thumbnail text, canBumpFee integer);
	create table inventory (invID text primary key not null, slug text, variantIndex integer, count integer);
	create index index_inventory on inventory (slug);
	create table purchases (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, vendorID text, vendorHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob);
	create index index_purchases on purchases (paymentAddr, timestamp);
	create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, needsSync integer);
	create index index_sales on sales (paymentAddr, timestamp);
	create table watchedscripts (scriptPubKey text primary key not null);
	create table cases (caseID text primary key not null, buyerContract blob, vendorContract blob, buyerValidationErrors blob, vendorValidationErrors blob, buyerPayoutAddress text, vendorPayoutAddress text, buyerOutpoints blob, vendorOutpoints blob, state integer, read integer, timestamp integer, buyerOpened integer, claim text, disputeResolution blob);
	create index index_cases on cases (timestamp);
	create table chat (messageID text primary key not null, peerID text, subject text, message text, read integer, timestamp integer, outgoing integer);
	create index index_chat on chat (peerID, subject, read, timestamp);
	create table notifications (notifID text primary key not null, serializedNotification blob, type text, timestamp integer, read integer);
	create index index_notifications on notifications (read, type, timestamp);
	create table coupons (slug text, code text, hash text);
	create index index_coupons on coupons (slug);
	create table moderatedstores (peerID text primary key not null);
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

func (c *ConfigDB) Init(mnemonic string, identityKey []byte, password string, creationDate time.Time) error {
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
	c.lock.Lock()
	defer c.lock.Unlock()
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
	c.lock.Lock()
	defer c.lock.Unlock()
	pwdCheck := "select count(*) from sqlite_master;"
	_, err := c.db.Exec(pwdCheck) // Fails if wrong password is entered
	if err != nil {
		return true
	}
	return false
}
