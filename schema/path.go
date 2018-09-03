package schema

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mitchellh/go-homedir"
)

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

// DefaultPathTransform accepts a string path representing the location where
// application data can be stored and returns a string representing the location
// where OpenBazaar prefers to store its schema on the filesystem relative to that
// path. If the path cannot be transformed, an error will be returned
func OpenbazaarPathTransform(basePath string, testModeEnabled bool) (path string, err error) {
	path, err = homedir.Expand(filepath.Join(basePath, directoryName(testModeEnabled)))
	if err == nil {
		path = filepath.Clean(path)
	}
	return path, err
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
	return directoryName
}
