package schema

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/mitchellh/go-homedir"
)

const minOBRepoVer = 16

func defaultDataPath() (path string) {
	if runtime.GOOS == "darwin" {
		return "~/Library/Application Support"
	}
	return "~"
}

// GenerateTempPath returns a string path representing the location where
// it is okay to store temporary data. No structure or created or deleted as
// part of this operation.
func GenerateTempPath() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return filepath.Join(os.TempDir(), fmt.Sprintf("ob_tempdir_%d", r.Intn(999)))
}

// OpenbazaarPathTransform accepts a string path representing the location where
// application data can be stored and returns a string representing the location
// where OpenBazaar prefers to store its schema on the filesystem relative to that
// path. If the path cannot be transformed, an error will be returned
func OpenbazaarPathTransform(basePath string, testModeEnabled bool) (path string, err error) {
	// First check to see if the .openbazaar directory exists and has the correct repo version
	obPath := filepath.Join(basePath, directoryName(testModeEnabled))
	versionBytes, ferr := ioutil.ReadFile(filepath.Join(obPath, "repover"))
	version, serr := strconv.Atoi(string(versionBytes))
	if ferr != nil || serr != nil || version < minOBRepoVer {
		// .openbazaar either didn't exist or was not migrated so let's see if the .openbazaar2.0 exists
		legacyPath := filepath.Join(basePath, legacyDirectoryName(testModeEnabled))
		if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
			// The legacy directory exists so let's use that. This should trigger a migration up
			// to the new directory location.
			obPath = legacyPath
		}
	}
	return expandPath(obPath)
}

func NewDataDirPathTransform(basePath string, testModeEnabled bool) (path string, err error) {
	return expandPath(filepath.Join(basePath, directoryName(testModeEnabled)))
}

func LegacyDataDirPathTransform(basePath string, testModeEnabled bool) (path string, err error) {
	return expandPath(filepath.Join(basePath, legacyDirectoryName(testModeEnabled)))
}

func legacyDirectoryName(isTestnet bool) (directoryName string) {
	if runtime.GOOS == "linux" {
		directoryName = ".openbazaar2.0"
	} else {
		directoryName = "OpenBazaar2.0"
	}

	if isTestnet {
		directoryName += "-testnet"
	}
	return directoryName
}

func directoryName(isTestnet bool) (directoryName string) {
	if runtime.GOOS == "linux" {
		directoryName = ".openbazaar"
	} else {
		directoryName = "OpenBazaar"
	}

	if isTestnet {
		directoryName += "-testnet"
	}
	return directoryName
}

func expandPath(pth string) (string, error) {
	path, err := homedir.Expand(pth)
	if err == nil {
		path = filepath.Clean(path)
	}
	return path, err
}
