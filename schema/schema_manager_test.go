package schema

import (
	"database/sql"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestNewSchemaManagerSetsReasonableDefaults(t *testing.T) {
	subject, err := NewSchemaManager()
	if err != nil {
		t.Fatal(err)
	}
	if subject.testModeEnabled != false {
		t.Error("Expected test mode to be disabled by default")
	}
	if subject.os != runtime.GOOS {
		t.Error("Expected default OS to be set via runtime.GOOS constant")
	}

	expectedDataPath := "/foobarbaz"
	subject, err = NewCustomSchemaManager(SchemaContext{
		DataPath:        expectedDataPath,
		TestModeEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if subject.testModeEnabled != true {
		t.Error("Expected test mode to be enabled")
	}
	if strings.HasPrefix(subject.DataPath(), expectedDataPath) != true {
		t.Errorf("Expected DataPath to start with %s", expectedDataPath)
	}
}

func TestNewSchemaManagerServiceDatastorePath(t *testing.T) {
	dataPath := "/root"
	subject, err := NewCustomSchemaManager(SchemaContext{
		DataPath:        dataPath,
		TestModeEnabled: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	expectedDatastorePath := "/root/datastore/mainnet.db"
	actualPath := subject.DatastorePath()
	if actualPath != expectedDatastorePath {
		t.Errorf("Datastore path for test disabled was incorrect\n\tExpected: %s\n\tActual: %s",
			expectedDatastorePath,
			actualPath,
		)
	}

	subject, err = NewCustomSchemaManager(SchemaContext{
		DataPath:        dataPath,
		TestModeEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	expectedDatastorePath = "/root/datastore/testnet.db"
	actualPath = subject.DatastorePath()
	if actualPath != expectedDatastorePath {
		t.Errorf("Datastore path for test enabled was incorrect\n\tExpected: %s\n\tActual: %s",
			expectedDatastorePath,
			actualPath,
		)
	}
}

func TestVerifySchemaVersion(t *testing.T) {
	var (
		expectedVersion = "123"
	)
	paths, err := NewCustomSchemaManager(SchemaContext{TestModeEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(paths.DataPath(), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	versionFile, err := os.Create(paths.DataPathJoin("repover"))
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

	if err = os.Remove(paths.DataPathJoin("repover")); err != nil {
		t.Fatal(err)
	}
	if err = paths.VerifySchemaVersion("missingfile!"); err == nil {
		t.Fatal("Expected schema version verification to fail when file is missing. Error:", err)
	}
}

func TestBuildSchemaDirectories(t *testing.T) {
	paths, err := NewCustomSchemaManager(SchemaContext{
		DataPath:        GenerateTempPath(),
		TestModeEnabled: true,
	})
	err = paths.BuildSchemaDirectories()
	if err != nil {
		t.Fatal(err)
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

func TestInitializeDatabaseSQL(t *testing.T) {
	database, _ := sql.Open("sqlite3", ":memory:")
	if _, err := database.Exec(InitializeDatabaseSQL("foobarbaz")); err != nil {
		t.Fatal("Expected InitializeDatabaseSQL to return executeable SQL, but got error:", err.Error())
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
