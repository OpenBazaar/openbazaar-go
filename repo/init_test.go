package repo

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"testing"
)

const repoRootFolder = "testdata/repo-root"
const mnemonicFixture = "fiscal first first inside toe wedding away element response dry attend oxygen"

// We have to use this and cannot pass the real db.Init because it would create a circular import error
func MockDbInit(mnemonic string, identityKey []byte, password string) error {
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
	password := "password"
	mnemonic := ""
	testnet := true
	// Running DoInit on a folder that already contains a config file
	err := DoInit(testConfigFolder, 4096, testnet, password, mnemonic, MockDbInit)
	if err != ErrRepoExists {
		t.Error("DoInit did not throw expected error")
	}
	// Running DoInit on an empty, not-writable folder
	os.Chmod(repoRootFolder, 0444)
	err = DoInit(repoRootFolder, 4096, testnet, password, mnemonic, MockDbInit)
	if err == nil {
		t.Error("DoInit did not throw an error")
	}
	// Running DoInit on an empty, writable folder
	os.Chmod(repoRootFolder, 0755)
	err = DoInit(repoRootFolder, 4096, testnet, password, mnemonic, MockDbInit)
	if err != nil {
		t.Errorf("DoInit threw an unexpected error: %s", err.Error())
	}

	TearDown()
}

func TestMaybeCreateOBDirectories(t *testing.T) {
	maybeCreateOBDirectories(repoRootFolder)
	checkDirectoryCreation(t, path.Join(repoRootFolder, "root"))
	checkDirectoryCreation(t, path.Join(repoRootFolder, "root", "listings"))
	checkDirectoryCreation(t, path.Join(repoRootFolder, "root", "feed"))
	checkDirectoryCreation(t, path.Join(repoRootFolder, "root", "channel"))
	checkDirectoryCreation(t, path.Join(repoRootFolder, "root", "files"))
	checkDirectoryCreation(t, path.Join(repoRootFolder, "root", "images"))
	checkDirectoryCreation(t, path.Join(repoRootFolder, "root", "images", "tiny"))
	checkDirectoryCreation(t, path.Join(repoRootFolder, "root", "images", "small"))
	checkDirectoryCreation(t, path.Join(repoRootFolder, "root", "images", "medium"))
	checkDirectoryCreation(t, path.Join(repoRootFolder, "root", "images", "large"))
	checkDirectoryCreation(t, path.Join(repoRootFolder, "root", "images", "original"))
	checkDirectoryCreation(t, path.Join(repoRootFolder, "outbox"))
	TearDown()
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
		t.Error("createMnemonic did not throw an error")
	}
}

// Removes files that are created when tests are executed
func TearDown() {
	os.RemoveAll(filepath.Join(testConfigFolder, "outbox"))
	os.RemoveAll(filepath.Join(testConfigFolder, "root"))
	os.RemoveAll(filepath.Join(testConfigFolder, "datastore"))
	os.Remove(filepath.Join(testConfigFolder, "repo.lock"))

	os.RemoveAll(filepath.Join(repoRootFolder, "blocks"))
	os.RemoveAll(filepath.Join(repoRootFolder, "outbox"))
	os.RemoveAll(filepath.Join(repoRootFolder, "root"))
	os.RemoveAll(filepath.Join(repoRootFolder, "datastore"))
	os.Remove(filepath.Join(repoRootFolder, "repo.lock"))
	os.Remove(filepath.Join(repoRootFolder, "config"))
	os.Remove(filepath.Join(repoRootFolder, "version"))
}
