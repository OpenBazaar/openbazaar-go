package schema

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
	_ "github.com/mutecomm/go-sqlcipher"
)

type openbazaarSchemaManager struct {
	dataPath        string
	db              *sql.DB
	os              string
	schemaPassword  string
	testModeEnabled bool
}

// SchemaContext are the parameters which the SchemaManager derive its source of
// truth. When their zero values are provided, a reasonable default will be
// assumed during runtime.
type SchemaContext struct {
	DataPath        string
	OS              string
	SchemaPassword  string
	TestModeEnabled bool
}

// DefaultPathTransform accepts a string path representing the location where
// application data can be stored and returns a string representing the location
// where OpenBazaar prefers to store its schema on the filesystem relative to that
// path. If the path cannot be transformed, an error will be returned
func OpenbazaarPathTransform(basePath string, testModeEnabled bool) (path string, err error) {
	path, err = homedir.Expand(filepath.Join(basePath, directoryName(testModeEnabled)))
	if err == nil {
		path = filepath.Clean(path)
	}
	return
}

// GenerateTempPath returns a string path representing the location where
// it is okay to store temporary data. No structure or created or deleted as
// part of this operation.
func GenerateTempPath() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return filepath.Join(os.TempDir(), fmt.Sprintf("ob_tempdir_%d", r.Intn(999)))
}

// NewSchemaManager returns a service that handles the data storage directory
// required during runtime. This service also ensures no errors can be produced
// at runtime after initial creation. An error may be produced if the SchemaManager
// is unable to verify the availability of the data storage directory.
func NewSchemaManager() (*openbazaarSchemaManager, error) {
	transformedPath, err := OpenbazaarPathTransform(defaultDataPath(), false)
	if err != nil {
		return nil, err
	}
	return NewCustomSchemaManager(SchemaContext{
		DataPath:        transformedPath,
		TestModeEnabled: false,
		OS:              runtime.GOOS,
	})
}

// NewCustomSchemaManger allows a custom SchemaContext to be provided to change
func NewCustomSchemaManager(ctx SchemaContext) (*openbazaarSchemaManager, error) {
	if len(ctx.DataPath) == 0 {
		path, err := OpenbazaarPathTransform(defaultDataPath(), ctx.TestModeEnabled)
		if err != nil {
			return nil, err
		}
		ctx.DataPath = path
	}
	if len(ctx.OS) == 0 {
		ctx.OS = runtime.GOOS
	}

	return &openbazaarSchemaManager{
		dataPath:        ctx.DataPath,
		os:              ctx.OS,
		schemaPassword:  ctx.SchemaPassword,
		testModeEnabled: ctx.TestModeEnabled,
	}, nil
}

func defaultDataPath() (path string) {
	if runtime.GOOS == "darwin" {
		return "~/Library/Application Support"
	}
	return "~"
}

func directoryName(isTestnet bool) (directoryName string) {
	if runtime.GOOS == "linux" {
		directoryName = ".openbazaar2.0"
	} else {
		directoryName = "OpenBazaar2.0"
	}

	if isTestnet {
		directoryName += "-testnet"
	}
	return
}

// DataPath returns the expected location of the data storage directory
func (m *openbazaarSchemaManager) DataPath() string { return m.dataPath }

// DatastorePath returns the expected location of the datastore file
func (m *openbazaarSchemaManager) DatastorePath() string {
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
	schemaVersion, err := ioutil.ReadFile(m.DataPathJoin("repover"))
	if err != nil {
		return fmt.Errorf("Accessing schema version: %s", err.Error())
	}
	if string(schemaVersion) != expectedVersion {
		return fmt.Errorf("Schema does not match expected version %s", expectedVersion)
	}
	return nil
}

// BuildSchemaDirectories creates the underlying directory structure required during runtime
func (m *openbazaarSchemaManager) BuildSchemaDirectories() error {
	if err := os.MkdirAll(m.DataPathJoin("datastore"), os.ModePerm); err != nil {
		return err
	}
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
	if err := os.MkdirAll(m.DataPathJoin("outbox"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.DataPathJoin("logs"), os.ModePerm); err != nil {
		return err
	}
	return nil
}

// DestroySchemaDirectories removes all schema files and folders permitted by the runtime
func (m *openbazaarSchemaManager) DestroySchemaDirectories() {
	os.RemoveAll(m.dataPath)
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
		CreateTableUnspentTransactionOutputsSQL,
		CreateTableSpentTransactionOutputsSQL,
		CreateTableTransactionsSQL,
		CreateTableTransactionMetadataSQL,
		CreateTableInventorySQL,
		CreateIndexInventorySQL,
		CreateTablePurchasesSQL,
		CreateIndexPurchasesSQL,
		CreateTableSalesSQL,
		CreateIndexSalesSQL,
		CreatedTableWatchedScriptsSQL,
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
