package schema

import (
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/tyler-smith/go-bip39"
)

const (
	CurrentSchemaVersion  = "8"
	IdentityKeyLength     = 4096
	DefaultSeedPassphrase = "Secret Passphrase"
)

type openbazaarSchemaManager struct {
	database        *sql.DB
	dataPath        string
	identityKey     []byte
	mnemonic        string
	os              string
	schemaPassword  string
	testModeEnabled bool
}

// SchemaContext are the parameters which the SchemaManager derive its source of
// truth. When their zero values are provided, a reasonable default will be
// assumed during runtime.
type SchemaContext struct {
	DataPath        string
	IdentityKey     []byte
	Mnemonic        string
	OS              string
	SchemaPassword  string
	TestModeEnabled bool
}

// NewSchemaManager returns a service that handles the data storage directory
// required during runtime. This service also ensures no errors can be produced
// at runtime after initial creation. An error may be produced if the SchemaManager
// is unable to verify the availability of the data storage directory.
func NewSchemaManager() (*openbazaarSchemaManager, error) {
	return NewCustomSchemaManager(SchemaContext{})
}

// NewCustomSchemaManger allows a custom SchemaContext to be provided
func NewCustomSchemaManager(ctx SchemaContext) (*openbazaarSchemaManager, error) {
	if len(ctx.DataPath) == 0 {
		path, err := OpenbazaarPathTransform(defaultDataPath(), ctx.TestModeEnabled)
		if err != nil {
			return nil, fmt.Errorf("finding root path: %s", err.Error())
		}
		ctx.DataPath = path
	}
	if len(ctx.OS) == 0 {
		ctx.OS = runtime.GOOS
	}
	if len(ctx.Mnemonic) == 0 {
		newMnemonic, err := NewMnemonic()
		if err != nil {
			return nil, err
		}
		ctx.Mnemonic = newMnemonic
	}

	if len(ctx.IdentityKey) == 0 {
		identityKey, err := CreateIdentityKey(ctx.Mnemonic)
		if err != nil {
			return nil, fmt.Errorf("generating identity: %s", err.Error())
		}
		ctx.IdentityKey = identityKey
	}

	return &openbazaarSchemaManager{
		dataPath:        ctx.DataPath,
		identityKey:     ctx.IdentityKey,
		mnemonic:        ctx.Mnemonic,
		os:              ctx.OS,
		schemaPassword:  ctx.SchemaPassword,
		testModeEnabled: ctx.TestModeEnabled,
	}, nil
}

// MustNewCustomSchemaManager returns a new schema manager or panics
func MustNewCustomSchemaManager(ctx SchemaContext) *openbazaarSchemaManager {
	if m, err := NewCustomSchemaManager(ctx); err != nil {
		panic(err)
	} else {
		return m
	}
}

// IsInitialized returns a bool indicating if the schema is ready to be used
func (m *openbazaarSchemaManager) IsInitialized() bool {
	return m.isDatabaseInitialized() && m.isConfigInitialized()
}

func (m *openbazaarSchemaManager) isDatabaseInitialized() bool {
	if err := m.VerifySchemaVersion(CurrentSchemaVersion); err != nil {
		return false
	}
	return true
}
func (m *openbazaarSchemaManager) isConfigInitialized() bool {
	return fsrepo.IsInitialized(m.DataPath())
}

// DataPath returns the expected location of the data storage directory
func (m *openbazaarSchemaManager) DataPath() string { return m.dataPath }

// Mnemonic returns the configured mnemonic used to generate the
// identity key used by the schema
func (m *openbazaarSchemaManager) Mnemonic() string { return m.mnemonic }

// IdentityKey returns the identity key used by the schema
func (m *openbazaarSchemaManager) IdentityKey() []byte { return m.identityKey }

// Identity returns the struct representation of the []byte IdentityKey
func (m *openbazaarSchemaManager) Identity() (*config.Identity, error) {
	if len(m.identityKey) == 0 {
		// All public constuctors set this value and should not occur during runtime
		return nil, errors.New("identity key is not generated")
	}
	identity, err := ipfs.IdentityFromKey(m.identityKey)
	if err != nil {
		return nil, err
	}
	return &identity, nil
}

// SchemaVersionFilePath returns the expect location of the schem version file
func (m *openbazaarSchemaManager) SchemaVersionFilePath() string {
	return m.DataPathJoin("repover")
}

// DatabasePath returns the expected location of the database file
func (m *openbazaarSchemaManager) DatabasePath() string {
	if m.testModeEnabled {
		return m.DataPathJoin("datastore", "testnet.db")
	}
	return m.DataPathJoin("datastore", "mainnet.db")
}

// DataPathJoin is a helper function which joins the pathArgs to the service's
// dataPath and returns the result
func (m *openbazaarSchemaManager) DataPathJoin(pathArgs ...string) string {
	allPathArgs := append([]string{m.dataPath}, pathArgs...)
	return filepath.Join(allPathArgs...)
}

// VerifySchemaVersion will ensure that the schema is currently
// the same as the expectedVersion otherwise returning an error. If the
// schema is exactly the same, nil will be returned.
func (m *openbazaarSchemaManager) VerifySchemaVersion(expectedVersion string) error {
	schemaVersion, err := ioutil.ReadFile(m.SchemaVersionFilePath())
	if err != nil {
		return fmt.Errorf("Accessing schema version: %s", err.Error())
	}
	if string(schemaVersion) != expectedVersion {
		return fmt.Errorf("Schema does not match expected version %s", expectedVersion)
	}
	return nil
}

// BuildSchemaDirectories creates the underlying schema structure required during runtime
func (m *openbazaarSchemaManager) BuildSchemaDirectories() error {
	if err := os.MkdirAll(m.DataPathJoin("datastore"), os.ModePerm); err != nil {
		return err
	}
	if err := m.buildIPFSRootDirectories(); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("outbox"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("logs"), os.ModePerm); err != nil {
		return err
	}
	return nil
}

func (m *openbazaarSchemaManager) buildIPFSRootDirectories() error {
	if err := os.MkdirAll(m.DataPathJoin("root"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("root", "listings"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("root", "ratings"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("root", "images"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("root", "images", "tiny"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("root", "images", "small"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("root", "images", "medium"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("root", "images", "large"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("root", "images", "original"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("root", "feed"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("root", "posts"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("root", "channel"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("root", "files"), os.ModePerm); err != nil {
		return err
	}
	return nil
}

// DestroySchemaDirectories removes all schema files and folders permitted by the runtime
func (m *openbazaarSchemaManager) DestroySchemaDirectories() {
	if err := os.RemoveAll(m.dataPath); err != nil {
		fmt.Printf("failed removing path '%s': %s\n", m.dataPath, err.Error())
	}
}

// ResetForJSONApiTest will reset the internal set of the schema without disturbing the
// node running on top of it
func (m *openbazaarSchemaManager) ResetForJSONApiTest() error {
	if m.testModeEnabled == false {
		return errors.New("destroy schema directories bypassed: must run while TestModeEnabled is true")
	}

	if db, err := m.OpenDatabase(); err != nil {
		return fmt.Errorf("opening database: %s", err.Error())
	} else {
		if _, err := db.Exec("delete from config where key = ?", "settings"); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(m.DataPathJoin("root")); err != nil {
		return fmt.Errorf("failed removing path '%s': %s\n", m.DataPathJoin("root"), err.Error())
	}
	if err := m.buildIPFSRootDirectories(); err != nil {
		return fmt.Errorf("building IPFS root: %s", err.Error())
	}

	return nil
}

// InitializeDatabaseSQL returns the executeable SQL string which initializes
// the database schema. It assumes the target is an empty SQLite3 database which
// supports encryption via the `PRAGMA key` statement
func InitializeDatabaseSQL(encryptionPassword string) string {
	initializeStatement := []string{
		PragmaKey(encryptionPassword),
		PragmaUserVersionSQL,
		CreateTableConfigSQL,
		CreateTableFollowersSQL,
		CreateTableFollowingSQL,
		CreateTableOfflineMessagesSQL,
		CreateTablePointersSQL,
		CreateTableKeysSQL,
		CreateIndexKeysSQL,
		CreateTableUnspentTransactionOutputsSQL,
		CreateIndexUnspentTransactionOutputsSQL,
		CreateTableSpentTransactionOutputsSQL,
		CreateIndexSpentTransactionOutputsSQL,
		CreateTableTransactionsSQL,
		CreateIndexTransactionsSQL,
		CreateTableTransactionMetadataSQL,
		CreateTableInventorySQL,
		CreateIndexInventorySQL,
		CreateTablePurchasesSQL,
		CreateIndexPurchasesSQL,
		CreateTableSalesSQL,
		CreateIndexSalesSQL,
		CreatedTableWatchedScriptsSQL,
		CreateIndexWatchedScriptsSQL,
		CreateTableDisputedCasesSQL,
		CreateIndexDisputedCasesSQL,
		CreateTableChatSQL,
		CreateIndexChatSQL,
		CreateTableNotificationsSQL,
		CreateIndexNotificationsSQL,
		CreateTableCouponsSQL,
		CreateIndexCouponsSQL,
		CreateTableModeratedStoresSQL,
	}
	return strings.Join(initializeStatement, " ")
}

// PragmaKey returns the executable SQL string to encrypt the database
func PragmaKey(password string) string {
	if len(password) == 0 {
		return ""
	}
	return fmt.Sprintf("pragma key = '%s';", password)
}

// OpenDatabase will internally setup the database and ensure it responds
// to ping.
func (m *openbazaarSchemaManager) OpenDatabase() (*sql.DB, error) {
	if m.database == nil {
		db, err := openAndPingDatabase(m.DatabasePath())
		if err != nil {
			return nil, err
		}
		m.database = db
	}

	return m.database, nil
}

//func (m *openbazaarSchemaManager) MigrateDatabase() error {
//return MigrateUp(m.DataPath(), m.schemaPassword, m.testModeEnabled)
//}

// InitializeDatabase creates the current schema's tables and sets
// the current schema version in the version file
func (m *openbazaarSchemaManager) InitializeDatabase() error {
	db, err := openAndPingDatabase(m.DatabasePath())
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec(InitializeDatabaseSQL(m.schemaPassword))
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	f, err := os.Create(m.SchemaVersionFilePath())
	if err != nil {
		return err
	}
	_, werr := f.Write([]byte(CurrentSchemaVersion))
	if werr != nil {
		return werr
	}
	f.Close()
	return nil
}

func openAndPingDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// InitializeIPFSRepo will create a default schema in the appropriate place in the
// root directory
func (m *openbazaarSchemaManager) InitializeIPFSRepo() error {
	db, err := openAndPingDatabase(m.DatabasePath())
	if err != nil {
		return fmt.Errorf("inititalize config: %s", err.Error())
	}
	if _, err := db.Exec("select count(*) from config"); err != nil {
		return fmt.Errorf("initialize config: database table not ready: %s", err.Error())
	}

	conf := MustDefaultConfig()
	identity, err := m.Identity()
	if err != nil {
		return err
	} else {
		conf.Identity = *identity
	}
	if err := m.initializeIPFSDirectoryWithConfig(conf); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	insertConfigRow, err := tx.Prepare("insert into config(key, value) values(?,?)")
	if err != nil {
		return err
	}
	defer insertConfigRow.Close()

	_, err = insertConfigRow.Exec("mnemonic", m.Mnemonic())
	if err != nil {
		tx.Rollback()
		return err
	}
	_, err = insertConfigRow.Exec("identityKey", m.IdentityKey())
	if err != nil {
		tx.Rollback()
		return err
	}
	_, err = insertConfigRow.Exec("creationDate", time.Now().Format(time.RFC3339))
	if err != nil {
		tx.Rollback()
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (m *openbazaarSchemaManager) initializeIPFSDirectoryWithConfig(c *config.Config) error {
	return fsrepo.Init(m.DataPath(), c)
}

// IdentityKey will return a []byte representing a node's verifiable identity
// based on the provided mnemonic string. If the string is empty, it will return
// an error
func CreateIdentityKey(mnemonic string) ([]byte, error) {
	if len(mnemonic) == 0 {
		return nil, ErrorEmptyMnemonic
	}
	seed := bip39.NewSeed(mnemonic, DefaultSeedPassphrase)
	identityKey, err := ipfs.IdentityKeyFromSeed(seed, IdentityKeyLength)
	if err != nil {
		return nil, err
	}
	return identityKey, nil
}

// NewMnemonic will return a randomly-generated BIP-39 compliant mnemonic
func NewMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		return "", fmt.Errorf("creating entropy: %s", err.Error())
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("generating mnemonic: %s", err.Error())
	}
	return mnemonic, nil
}

func MustDefaultConfig() *config.Config {
	bootstrapPeers, err := config.ParseBootstrapPeers(BootstrapAddressesDefault)
	if err != nil {
		// BootstrapAddressesDefault are local and should never panic
		panic(err)
	}

	conf := &config.Config{
		// Setup the node's default addresses.
		// NOTE: two swarm listen addrs, one TCP, one UTP.
		Addresses: config.Addresses{
			Swarm: []string{
				"/ip4/0.0.0.0/tcp/4001",
				"/ip6/::/tcp/4001",
				"/ip4/0.0.0.0/tcp/9005/ws",
				"/ip6/::/tcp/9005/ws",
			},
			API:     "",
			Gateway: "/ip4/127.0.0.1/tcp/4002",
		},

		Datastore: config.Datastore{
			StorageMax:         "10GB",
			StorageGCWatermark: 90, // 90%
			GCPeriod:           "1h",
			BloomFilterSize:    0,
			HashOnRead:         false,
			Spec: map[string]interface{}{
				"type": "mount",
				"mounts": []interface{}{
					map[string]interface{}{
						"mountpoint": "/blocks",
						"type":       "measure",
						"prefix":     "flatfs.datastore",
						"child": map[string]interface{}{
							"type":      "flatfs",
							"path":      "blocks",
							"sync":      true,
							"shardFunc": "/repo/flatfs/shard/v1/next-to-last/2",
						},
					},
					map[string]interface{}{
						"mountpoint": "/",
						"type":       "measure",
						"prefix":     "leveldb.datastore",
						"child": map[string]interface{}{
							"type":        "levelds",
							"path":        "datastore",
							"compression": "none",
						},
					},
				},
			},
		},
		Bootstrap: config.BootstrapPeerStrings(bootstrapPeers),
		Discovery: config.Discovery{config.MDNS{
			Enabled:  false,
			Interval: 10,
		}},

		// Setup the node mount points
		Mounts: config.Mounts{
			IPFS: "/ipfs",
			IPNS: "/ipns",
		},

		Ipns: config.Ipns{
			ResolveCacheSize:   128,
			RecordLifetime:     "7d",
			RepublishPeriod:    "24h",
			QuerySize:          5,
			UsePersistentCache: true,
		},

		Gateway: config.Gateway{
			RootRedirect: "",
			Writable:     false,
			PathPrefixes: []string{},
		},
	}

	return conf
}
