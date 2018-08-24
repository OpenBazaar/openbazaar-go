package repo

import (
	"errors"
	"os"
	"path/filepath"
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

func TestCreateMnemonic(t *testing.T) {
	mnemonic, err := createMnemonic(MockNewEntropyFail, MockNewMnemonicFail)
	checkCreateMnemonicError(t, mnemonic, err)
	mnemonic, err = createMnemonic(MockNewEntropy, MockNewMnemonicFail)
	checkCreateMnemonicError(t, mnemonic, err)
	mnemonic, err = createMnemonic(MockNewEntropy, MockNewMnemonic)
	if mnemonic != mnemonicFixture {
		t.Errorf("The mnemonic should have been %s but it is %s instead", mnemonicFixture, mnemonic)
	}
	if err != nil {
		t.Error("createMnemonic threw an unexpected error")
	}
}

func checkCreateMnemonicError(t *testing.T, mnemonic string, err error) {
	if mnemonic != "" {
		t.Errorf("The mnemonic should have been an empty string but it is %s instead", mnemonic)
	}
	if err == nil {
		t.Error("createMnemonic didn't throw an error")
	}
}

// Removes files that are created when tests are executed
func TearDown() {
	os.RemoveAll(filepath.Join("testdata", "outbox"))
	os.RemoveAll(filepath.Join("testdata", "root"))
	os.RemoveAll(filepath.Join("testdata", "datastore"))
	os.Remove(filepath.Join("testdata", "repo.lock"))

	os.RemoveAll(filepath.Join(repoRootFolder, "blocks"))
	os.RemoveAll(filepath.Join(repoRootFolder, "outbox"))
	os.RemoveAll(filepath.Join(repoRootFolder, "root"))
	os.RemoveAll(filepath.Join(repoRootFolder, "datastore"))
	os.Remove(filepath.Join(repoRootFolder, "repo.lock"))
	os.Remove(filepath.Join(repoRootFolder, "config"))
	os.Remove(filepath.Join(repoRootFolder, "version"))
}
