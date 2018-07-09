package repo

import (
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"io/ioutil"
	"os"
	"path"
	"strconv"
)

type Migration interface {
	Up(repoPath string, dbPassword string, testnet bool) error
	Down(repoPath string, dbPassword string, testnet bool) error
}

var Migrations = []Migration{
	migrations.Migration000{},
	migrations.Migration001{},
	migrations.Migration002{},
	migrations.Migration003{},
	migrations.Migration004{},
	migrations.Migration005{},
	migrations.Migration006{},
	migrations.Migration007{},
	migrations.Migration008{},
	migrations.Migration009{},
}

// MigrateUp looks at the currently active migration version
// and will migrate all the way up (applying all up migrations).
func MigrateUp(repoPath, dbPassword string, testnet bool) error {
	version, err := ioutil.ReadFile(path.Join(repoPath, "repover"))
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if err != nil && os.IsNotExist(err) {
		version = []byte("0")
	}
	v, err := strconv.Atoi(string(version))
	if err != nil {
		return err
	}
	x := v
	for _, m := range Migrations[v:] {
		log.Noticef("Migrating repo to version %d\n", x+1)
		err := m.Up(repoPath, dbPassword, testnet)
		if err != nil {
			log.Error(err)
			return err
		}
		x++
	}
	return nil
}
