package schema

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/tyler-smith/go-bip39"
)

func TestNewSchemaManagerSetsReasonableDefaults(t *testing.T) {
	subject, err := NewSchemaManager()
	if err != nil {
		t.Fatal(err)
	}
	if subject.databaseLock == nil {
		t.Error("Expected mutex lock to be initialized")
	}
	if subject.testModeEnabled != false {
		t.Error("Expected test mode to be disabled by default")
	}
	if subject.os != runtime.GOOS {
		t.Error("Expected default OS to be set via runtime.GOOS constant")
	}
	subjectMnemonic := subject.Mnemonic()
	if len(subjectMnemonic) == 0 {
		t.Error("Expected default mnemonic to be generated if not provided")
	}
	expectedIdentity, err := CreateIdentityKey(subjectMnemonic)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(expectedIdentity, subject.IdentityKey()) != true {
		t.Error("Expected IdentityKey() to provide the identity key for the testMnemonic but was different")
	}

	testMnemonic := "foo bar baz qux"
	expectedIdentity, err = CreateIdentityKey(testMnemonic)
	if err != nil {
		t.Fatal(err)
	}
	expectedDataPath := "/foobarbaz"
	subject = MustNewCustomSchemaManager(SchemaContext{
		DataPath:        expectedDataPath,
		TestModeEnabled: true,
		Mnemonic:        testMnemonic,
	})
	if subject.testModeEnabled != true {
		t.Error("Expected test mode to be enabled")
	}
	if strings.HasPrefix(subject.DataPath(), expectedDataPath) != true {
		t.Errorf("Expected DataPath to start with %s", expectedDataPath)
	}
	if len(subject.Mnemonic()) == 0 {
		t.Error("Expected mnemonic to be generated when not provided")
	}
	if bytes.Equal(expectedIdentity, subject.IdentityKey()) != true {
		t.Error("Expected IdentityKey() to provide the identity key for the testMnemonic but was different")
	}
}

func TestNewSchemaManagerServiceDatabasePath(t *testing.T) {
	dataPath := "/root"
	subject := MustNewCustomSchemaManager(SchemaContext{
		DataPath:        dataPath,
		TestModeEnabled: false,
	})
	expectedDatabasePath := "/root/datastore/mainnet.db"
	actualPath := subject.DatabasePath()
	if actualPath != expectedDatabasePath {
		t.Errorf("Database path for test disabled was incorrect\n\tExpected: %s\n\tActual: %s",
			expectedDatabasePath,
			actualPath,
		)
	}

	subject = MustNewCustomSchemaManager(SchemaContext{
		DataPath:        dataPath,
		TestModeEnabled: true,
	})
	expectedDatabasePath = "/root/datastore/testnet.db"
	actualPath = subject.DatabasePath()
	if actualPath != expectedDatabasePath {
		t.Errorf("Database path for test enabled was incorrect\n\tExpected: %s\n\tActual: %s",
			expectedDatabasePath,
			actualPath,
		)
	}
}

func TestMustDefaultConfig(t *testing.T) {
	config := MustDefaultConfig()
	if config == nil {
		t.Error("Expected config to not be empty")
	}
	if config.Addresses.Gateway != "/ip4/127.0.0.1/tcp/4002" {
		t.Error("config.Addresses.Gateway is not set")
	}
}

func TestSchemaManagerChecksIsInitialized(t *testing.T) {
	subject := MustNewCustomSchemaManager(SchemaContext{
		DataPath:        GenerateTempPath(),
		TestModeEnabled: true,
	})
	if subject.IsInitialized() != false {
		t.Error("Expected subject to not be initialized and return false")
	}

	if err := subject.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	if err := subject.InitializeDatabase(); err != nil {
		t.Fatal(err)
	}
	if subject.IsInitialized() != false {
		t.Error("Expected subject to not be initialized and return false")
	}

	err := subject.InitializeIPFSRepo()
	if err != nil {
		t.Fatal("Unable to initialize configuration file")
	}
	if subject.IsInitialized() != true {
		t.Error("Expected subject to be initialized (config is present and valid) and return true")
	}
	if len(subject.IdentityKey()) == 0 {
		t.Error("Expected InitConfig to generate an identity key")
	}
	identity, err := subject.Identity()
	if err != nil {
		t.Error(err)
	}
	if err == nil && identity.PeerID == "" {
		t.Error("Expected InitConfig to generate an identity")
	}
}

func TestVerifySchemaVersion(t *testing.T) {
	var (
		expectedVersion = "123"
	)
	paths := MustNewCustomSchemaManager(SchemaContext{TestModeEnabled: true})
	if err := os.MkdirAll(paths.DataPath(), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	versionFile, err := os.Create(paths.SchemaVersionFilePath())
	if err != nil {
		t.Fatal(err)
	}
	_, err = versionFile.Write([]byte(expectedVersion))
	if err != nil {
		t.Fatal(err)
	}
	versionFile.Close()

	if err = paths.VerifySchemaVersion(expectedVersion); err != nil {
		t.Fatal("Expected schema version verification to pass with expected version. Error:", err)
	}
	if err = paths.VerifySchemaVersion("anotherversion"); err == nil {
		t.Fatal("Expected schema version verification to fail with wrong version. Error:", err)
	}

	if err = os.Remove(paths.SchemaVersionFilePath()); err != nil {
		t.Fatal(err)
	}
	if err = paths.VerifySchemaVersion("missingfile!"); err == nil {
		t.Fatal("Expected schema version verification to fail when file is missing. Error:", err)
	}
}

func TestBuildSchemaDirectories(t *testing.T) {
	permissionlessPath := "/tmp/pathwithoutpermissions"
	if err := os.Mkdir(permissionlessPath, 0); err != nil {
		t.Fatal("Unable to create permissionless path:", err.Error())
	}
	paths := MustNewCustomSchemaManager(SchemaContext{
		DataPath:        permissionlessPath,
		TestModeEnabled: true,
	})
	if err := paths.BuildSchemaDirectories(); err != nil && os.IsPermission(err) == false {
		t.Error("Expected build directories to fail due to lack of permissions")
	}
	paths.DestroySchemaDirectories()

	if err := paths.BuildSchemaDirectories(); err != nil {
		t.Error("Expected build directories to work successfully, but did not:", err.Error())
	}
	defer paths.DestroySchemaDirectories()

	checkDirectoryCreation(t, paths.DataPathJoin("root"))
	checkDirectoryCreation(t, paths.DataPathJoin("root", "listings"))
	checkDirectoryCreation(t, paths.DataPathJoin("root", "feed"))
	checkDirectoryCreation(t, paths.DataPathJoin("root", "channel"))
	checkDirectoryCreation(t, paths.DataPathJoin("root", "files"))
	checkDirectoryCreation(t, paths.DataPathJoin("root", "images"))
	checkDirectoryCreation(t, paths.DataPathJoin("root", "images", "tiny"))
	checkDirectoryCreation(t, paths.DataPathJoin("root", "images", "small"))
	checkDirectoryCreation(t, paths.DataPathJoin("root", "images", "medium"))
	checkDirectoryCreation(t, paths.DataPathJoin("root", "images", "large"))
	checkDirectoryCreation(t, paths.DataPathJoin("root", "images", "original"))
	checkDirectoryCreation(t, paths.DataPathJoin("outbox"))
	checkDirectoryCreation(t, paths.DataPathJoin("logs"))
	checkDirectoryCreation(t, paths.DataPathJoin("datastore"))
}

func checkDirectoryCreation(t *testing.T, directory string) {
	f, err := os.Open(directory)
	if err != nil {
		t.Errorf("created directory %s could not be opened", directory)
	}
	fi, _ := f.Stat()
	if fi.IsDir() == false {
		t.Errorf("maybeCreateOBDirectories did not create the directory %s", directory)
	}
	if fi.Mode().String()[1:3] != "rw" {
		t.Errorf("the created directory %s is not readable and writable for the owner", directory)
	}
}

func TestCreateIdentityKey(t *testing.T) {
	_, err := CreateIdentityKey("")
	if err != ErrorEmptyMnemonic {
		t.Error("Expected empty mnemonic to return ErrorEmptyMnemonic")
	}

	testMnemonic := "this is a test mnemonic"
	testSeed := bip39.NewSeed(testMnemonic, DefaultSeedPassphrase)
	expectedIdentityKey, err := ipfs.IdentityKeyFromSeed(testSeed, IdentityKeyLength)
	if err != nil {
		t.Fatal("Unexpected error generating expected identity key")
	}

	actualIdentity, err := CreateIdentityKey(testMnemonic)
	if err != nil {
		t.Fatal("Unexpected error generating actual identity key")
	}
	if bytes.Equal(actualIdentity, expectedIdentityKey) != true {
		t.Error("Actual identity was different from expected identity")
	}
}

func TestInitializeDatabaseSQL(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	if _, err := database.Exec(InitializeDatabaseSQL("foobarbaz")); err != nil {
		t.Fatal("Expected InitializeDatabaseSQL to return executeable SQL, but got error:", err.Error())
	}
}

func TestInitializeDatabase(t *testing.T) {
	subject := MustNewCustomSchemaManager(SchemaContext{
		DataPath:        GenerateTempPath(),
		TestModeEnabled: true,
	})
	err := subject.InitializeDatabase()
	if err == nil {
		t.Fatal("Expected InitializeDatabase to fail when directories do not exist")
	}
	if strings.Contains(err.Error(), "unable to open database file") == false {
		t.Error("Expected error to indicate unable to open database file, received:", err.Error())
	}

	if err := subject.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	err = subject.InitializeDatabase()
	if err != nil {
		t.Fatal(err)
	}

	schemaTables := []string{
		"config",
		"followers",
		"following",
		"offlinemessages",
		"pointers",
		"keys",
		"utxos",
		"stxos",
		"txns",
		"txmetadata",
		"inventory",
		"purchases",
		"sales",
		"watchedscripts",
		"cases",
		"chat",
		"notifications",
		"coupons",
		"moderatedstores",
	}
	db, err := subject.OpenDatabase()
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range schemaTables {
		if _, err := db.Exec(fmt.Sprintf("select count(*) from %s", table)); err != nil {
			t.Errorf("Error accessing table '%s': %s", table, err.Error())
		}
	}

	if err := subject.VerifySchemaVersion(CurrentSchemaVersion); err != nil {
		t.Errorf("Expected starting schema version to be %s", CurrentSchemaVersion)
	}
}

func TestInitializeIPFSRepo(t *testing.T) {
	subject := MustNewCustomSchemaManager(SchemaContext{
		DataPath:        GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err := subject.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer subject.DestroySchemaDirectories()

	if err := subject.InitializeIPFSRepo(); err == nil {
		t.Error("Expecting InititalizeConfig to fail when database is not inititalized")
	}
	if err := subject.InitializeDatabase(); err != nil {
		t.Fatal(err)
	}

	if err := subject.InitializeIPFSRepo(); err != nil {
		t.Error(err)
	}

	db, err := subject.OpenDatabase()
	if err != nil {
		t.Fatal(err)
	}
	var mnemonicPresent, identityPresent, creationDatePresent bool
	rows, err := db.Query("select key, value from config")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var (
			key   string
			value []byte
		)
		if err := rows.Scan(&key, &value); err != nil {
			t.Error("error getting row:", err.Error())
		}
		if key == "mnemonic" {
			mnemonicPresent = true
			if string(value) != subject.Mnemonic() {
				t.Error("Unexpected mnemonic saved in database")
			}
		}
		if key == "identityKey" {
			identityPresent = true
			if bytes.Equal(value, subject.IdentityKey()) != true {
				t.Error("Unexpected identity key saved in database")
			}
		}
		if key == "creationDate" {
			creationDatePresent = true
			timeValue, err := time.Parse(time.RFC3339, string(value))
			if err != nil {
				t.Error("Unable to parse creationTime:", err.Error())
			}
			if time.Now().Sub(timeValue) > (time.Duration(5) * time.Second) {
				t.Error("Unexpected creationTime to be set within the last 5 seconds")
			}
		}
	}

	if mnemonicPresent == false {
		t.Error("Expected mnemonic key to be created in config table")
	}
	if identityPresent == false {
		t.Error("Expected identityKey key to be created in config table")
	}
	if creationDatePresent == false {
		t.Error("Expected creationDate key to be created in config table")
	}
}

const versionZeroSchema = `
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
  create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob);
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

func TestMigrateDatabase(t *testing.T) {
	appSchema := MustNewCustomSchemaManager(SchemaContext{
		DataPath:        GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err := appSchema.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer appSchema.DestroySchemaDirectories()

	db, err := appSchema.OpenDatabase()
	if err != nil {
		t.Fatal(err)
	}

	// Inititalize Database with Version 0 schema
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(versionZeroSchema); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := appSchema.InitializeIPFSRepo(); err != nil {
		t.Fatal(err)
	}

	// Override the repo schema version
	err = ioutil.WriteFile(appSchema.SchemaVersionFilePath(), []byte("0"), 0666)
	if err != nil {
		t.Fatal("Unexpected error creating pre-migrated schema version file")
	}

	if err := appSchema.MigrateDatabase(); err != nil {
		t.Fatal(err)
	}

	if err := appSchema.VerifySchemaVersion(CurrentSchemaVersion); err != nil {
		t.Error("Expected schema version to be updated to current version")
	}
}
