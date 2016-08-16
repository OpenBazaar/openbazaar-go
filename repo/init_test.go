package repo

import (
	"os"
	"path"
	"path/filepath"
	"testing"
)

const repoRootFolder = "testdata/repo-root"

// we have to use this and cannot pass the real db.Init because it would create a circular import error
func MockDbInit(mnemonic string, identityKey []byte, password string) error {
	return nil
}

func TestDoInit(t *testing.T) {
	password := "password"
	mnemonic := ""
	testnet := true
	// running DoInit on a folder that already contains a config file
	err := DoInit(testConfigFolder, 4096, testnet, password, mnemonic, MockDbInit)
	if err != ErrRepoExists {
		t.Error("DoInit didn't throw expected error")
	}
	// running DoInit on an empty, not-writable folder
	os.Chmod(repoRootFolder, 0444)
	err = DoInit(repoRootFolder, 4096, testnet, password, mnemonic, MockDbInit)
	if err == nil {
		t.Error("DoInit didn't throw an error")
	}
	// running DoInit on an empty, writable folder
	os.Chmod(repoRootFolder, 0755)
	err = DoInit(repoRootFolder, 4096, testnet, password, mnemonic, MockDbInit)
	if err != nil {
		t.Error("DoInit threw an unexpected error")
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

// removes files that are created when tests are executed
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
