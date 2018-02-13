package util

import (
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

	expectedRootPath := "/foobarbaz"
	subject, err = NewCustomSchemaManager(SchemaContext{
		RootPath:        expectedRootPath,
		TestModeEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if subject.testModeEnabled != true {
		t.Error("Expected test mode to be enabled")
	}
	if strings.HasPrefix(subject.RootPath(), expectedRootPath) != true {
		t.Errorf("Expected rootPath to start with %s", expectedRootPath)
	}
}

func TestNewSchemaManagerServiceDatastorePath(t *testing.T) {
	rootPath := "/root"
	subject, err := NewCustomSchemaManager(SchemaContext{
		RootPath:        rootPath,
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
		RootPath:        rootPath,
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

func TestMustVerifySchemaVersion(t *testing.T) {
	var (
		expectedVersion = "123"
	)
	paths, err := NewCustomSchemaManager(SchemaContext{TestModeEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(paths.RootPath(), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	versionFile, err := os.Create(paths.RootPathJoin("repover"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = versionFile.Write([]byte(expectedVersion))
	if err != nil {
		t.Fatal(err)
	}
	versionFile.Close()

	if err = paths.MustVerifySchemaVersion(expectedVersion); err != nil {
		t.Fatal("Expected schema version verification to pass with expected version. Error:", err)
	}
	if err = paths.MustVerifySchemaVersion("anotherversion"); err == nil {
		t.Fatal("Expected schema version verification to fail with wrong version. Error:", err)
	}

	if err = os.Remove(paths.RootPathJoin("repover")); err != nil {
		t.Fatal(err)
	}
	if err = paths.MustVerifySchemaVersion("missingfile!"); err == nil {
		t.Fatal("Expected schema version verification to fail when file is missing. Error:", err)
	}
}

func TestBuildSchemaDirectories(t *testing.T) {
	paths, err := NewCustomSchemaManager(SchemaContext{
		RootPath:        GenerateTempPath(),
		TestModeEnabled: true,
	})
	err = paths.BuildSchemaDirectories()
	if err != nil {
		t.Fatal(err)
	}
	defer paths.DestroySchemaDirectories()

	checkDirectoryCreation(t, paths.RootPathJoin("root"))
	checkDirectoryCreation(t, paths.RootPathJoin("root", "listings"))
	checkDirectoryCreation(t, paths.RootPathJoin("root", "feed"))
	checkDirectoryCreation(t, paths.RootPathJoin("root", "channel"))
	checkDirectoryCreation(t, paths.RootPathJoin("root", "files"))
	checkDirectoryCreation(t, paths.RootPathJoin("root", "images"))
	checkDirectoryCreation(t, paths.RootPathJoin("root", "images", "tiny"))
	checkDirectoryCreation(t, paths.RootPathJoin("root", "images", "small"))
	checkDirectoryCreation(t, paths.RootPathJoin("root", "images", "medium"))
	checkDirectoryCreation(t, paths.RootPathJoin("root", "images", "large"))
	checkDirectoryCreation(t, paths.RootPathJoin("root", "images", "original"))
	checkDirectoryCreation(t, paths.RootPathJoin("outbox"))
	checkDirectoryCreation(t, paths.RootPathJoin("logs"))
	checkDirectoryCreation(t, paths.RootPathJoin("datastore"))
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
