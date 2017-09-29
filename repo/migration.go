package repo

import (
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"io/ioutil"
	"os"
	"path"
	"strconv"
)

type Migration interface {
	Up(repoPath string) error
	Down(repoPath string) error
}

var Migrations = []Migration{
	migrations.Migration000,
	migrations.Migration001,
}

// MigrateUp looks at the currently active migration version
// and will migrate all the way up (applying all up migrations).
func MigrateUp(repoPath string) error {
	version, err := ioutil.ReadFile(path.Join(repoPath, "repover"))
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if err != nil && os.IsNotExist(err) {
		version = []byte("0")
	}
	v, err := strconv.Atoi(string(version[0]))
	if err != nil {
		return err
	}
	for _, m := range Migrations[v:] {
		log.Noticef("Migrationg repo to version %d\n", v+1)
		err := m.Up(repoPath)
		if err != nil {
			return err
		}
	}
	return nil
}
