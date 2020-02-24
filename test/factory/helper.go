package factory

import (
	"os"
	"path/filepath"
)

func fixtureLoadPath() string {
	gopath := os.Getenv("GOPATH")
	repoPath := filepath.Join("src", "github.com", "OpenBazaar", "openbazaar-go")
	fixturePath, err := filepath.Abs(filepath.Join(gopath, repoPath, "test", "factory", "fixtures"))
	if err != nil {
		panic("cannot create absolute path")
	}
	return fixturePath
}
