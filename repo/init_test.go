package repo

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/schema"
)

const repoRootFolder = "testdata/repo-root"
const mnemonicFixture = "fiscal first first inside toe wedding away element response dry attend oxygen"

// We have to use this and cannot pass the real db.Init because it would create a circular import error
func MockDbInit(mnemonic string, identityKey []byte, password string, creationDate time.Time) error {
	return nil
}
func MockNewEntropy(int) ([]byte, error) {
	entropy := make([]byte, 32)
	return entropy, nil
}
func MockNewEntropyFail(int) ([]byte, error) {
	entropy := make([]byte, 32)
	err := errors.New("")
	return entropy, err
}
func MockNewMnemonic([]byte) (string, error) {
	return mnemonicFixture, nil
}
func MockNewMnemonicFail([]byte) (string, error) {
	mnemonic := ""
	err := errors.New("")
	return mnemonic, err
}

func TestDoInit(t *testing.T) {
	var (
		password      = "password"
		mnemonic      = ""
		testnet       = true
		testDirectory = schema.GenerateTempPath()
	)
	paths, err := schema.NewCustomSchemaManager(schema.SchemaContext{DataPath: testDirectory})
	if err != nil {
		t.Fatal(err)
	}
	// Running DoInit on a folder that already contains a config file
	err = DoInit(paths.DataPath(), 4096, testnet, password, mnemonic, time.Now(), MockDbInit)
	if err != nil {
		t.Error("First DoInit should not have failed:", err.Error())
	}
	err = DoInit(paths.DataPath(), 4096, testnet, password, mnemonic, time.Now(), MockDbInit)
	if err != ErrRepoExists {
		t.Error("Expected DoInit to fail with ErrRepoExists but did not")
	}
	paths.DestroySchemaDirectories()
	if err = paths.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}

	// Running DoInit on an empty, writable folder
	if err = os.Chmod(paths.DataPath(), 0755); err != nil {
		t.Fatal(err)
	}
	err = DoInit(paths.DataPath(), 4096, testnet, password, mnemonic, time.Now(), MockDbInit)
	if err != nil {
		t.Errorf("DoInit threw an unexpected error: %s", err.Error())
	}
	paths.DestroySchemaDirectories()
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
