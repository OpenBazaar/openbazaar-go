package schema

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	bip39 "github.com/tyler-smith/go-bip39"
)

func TestMain(m *testing.M) {
	// The repo package usually installs the plugins
	// on init() but the repo package isn't initialized
	// here and it would be a circular import to import
	// it. So we will install the plugin for the purposes
	// of these tests.
	ipfs.InstallDatabasePlugins()
	os.Exit(m.Run())
}

func TestNewSchemaManagerSetsReasonableDefaults(t *testing.T) {
	subject, err := NewSchemaManager()
	if err != nil {
		t.Fatal(err)
	}
	if subject.testModeEnabled {
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
	if !bytes.Equal(expectedIdentity, subject.IdentityKey()) {
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
	if !subject.testModeEnabled {
		t.Error("Expected test mode to be enabled")
	}
	if !strings.HasPrefix(subject.DataPath(), expectedDataPath) {
		t.Errorf("Expected DataPath to start with %s", expectedDataPath)
	}
	if len(subject.Mnemonic()) == 0 {
		t.Error("Expected mnemonic to be generated when not provided")
	}
	if !bytes.Equal(expectedIdentity, subject.IdentityKey()) {
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
	if config != nil {
		if config.Addresses.Gateway[0] != "/ip4/127.0.0.1/tcp/4002" {
			t.Error("config.Addresses.Gateway is not set")
		}
	}
}

func TestSchemaManagerChecksIsInitialized(t *testing.T) {
	subject := MustNewCustomSchemaManager(SchemaContext{
		DataPath:        GenerateTempPath(),
		TestModeEnabled: true,
	})
	if subject.IsInitialized() {
		t.Error("Expected subject to not be initialized and return false")
	}

	if err := subject.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	if err := subject.InitializeDatabase(); err != nil {
		t.Fatal(err)
	}
	if subject.IsInitialized() {
		t.Error("Expected subject to not be initialized and return false")
	}

	err := subject.InitializeIPFSRepo()
	if err != nil {
		t.Fatal("Unable to initialize configuration file")
	}
	if !subject.IsInitialized() {
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
	if err := paths.BuildSchemaDirectories(); err != nil && !os.IsPermission(err) {
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
	if !fi.IsDir() {
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
	if !bytes.Equal(actualIdentity, expectedIdentityKey) {
		t.Error("Actual identity was different from expected identity")
	}
}

func TestInitializeDatabaseSQL(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	if _, err := database.Exec(InitializeDatabaseSQL("foobarbaz")); err != nil {
		t.Fatal("Expected InitializeDatabaseSQL to return executable SQL, but got error:", err.Error())
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
	if !strings.Contains(err.Error(), "unable to open database file") {
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
			if !bytes.Equal(value, subject.IdentityKey()) {
				t.Error("Unexpected identity key saved in database")
			}
		}
		if key == "creationDate" {
			creationDatePresent = true
			timeValue, err := time.Parse(time.RFC3339, string(value))
			if err != nil {
				t.Error("Unable to parse creationTime:", err.Error())
			}
			if time.Since(timeValue) > (time.Duration(5) * time.Second) {
				t.Error("Unexpected creationTime to be set within the last 5 seconds")
			}
		}
	}

	if !mnemonicPresent {
		t.Error("Expected mnemonic key to be created in config table")
	}
	if !identityPresent {
		t.Error("Expected identityKey key to be created in config table")
	}
	if !creationDatePresent {
		t.Error("Expected creationDate key to be created in config table")
	}
}

func TestOpenbazaarSchemaManager_CleanIdentityFromConfig(t *testing.T) {
	subject := MustNewCustomSchemaManager(SchemaContext{
		DataPath:        GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err := subject.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer subject.DestroySchemaDirectories()

	if err := subject.InitializeDatabase(); err != nil {
		t.Fatal(err)
	}

	if err := subject.InitializeIPFSRepo(); err != nil {
		t.Error(err)
	}

	loadConfig := func() (map[string]interface{}, error) {
		configPath := path.Join(subject.dataPath, "config")
		configFile, err := ioutil.ReadFile(configPath)
		if err != nil {
			return map[string]interface{}{}, err
		}
		var cfgIface interface{}
		if err := json.Unmarshal(configFile, &cfgIface); err != nil {
			return map[string]interface{}{}, err
		}
		cfg, ok := cfgIface.(map[string]interface{})
		if !ok {
			return map[string]interface{}{}, errors.New("invalid config file")
		}
		return cfg, nil
	}

	// First load the config and make sure the identity object is indeed set.

	cfg, err := loadConfig()
	if err != nil {
		t.Error("config can not be loaded")
	}
	_, ok := cfg["Identity"]
	if !ok {
		t.Error("identity object does not exist in config but should")
	}

	// Now clean and check again
	if err := subject.CleanIdentityFromConfig(); err != nil {
		t.Error(err)
	}
	cfg, err = loadConfig()
	if err != nil {
		t.Error("config can not be loaded")
	}
	_, ok = cfg["Identity"]
	if ok {
		t.Error("Identity object was not deleted from config")
	}
}
