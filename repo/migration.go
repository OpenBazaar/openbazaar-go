package repo

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
)

type Migration interface {
	Up(repoPath string, dbPassword string, testnet bool) error
	Down(repoPath string, dbPassword string, testnet bool) error
}

var (
	ErrUnknownSchema = errors.New("unable to migrate unknown schema")

	Migrations = []Migration{
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
		migrations.Migration010{},
		migrations.Migration011{},
		migrations.Migration012{},
		migrations.Migration013{},
		migrations.Migration014{},
		migrations.Migration015{},
		migrations.Migration016{},
		migrations.Migration017{},
		migrations.Migration018{},
		migrations.Migration019{},
		migrations.Migration020{},
		migrations.Migration021{},
		migrations.Migration022{},
		migrations.Migration023{},
		migrations.Migration024{},
		migrations.Migration025{},
		migrations.Migration026{},
		migrations.Migration027{},
		migrations.Migration028{},
		migrations.Migration029{},
		migrations.Migration030{},
		migrations.Migration031{},
		migrations.Migration032{},
		migrations.Migration033{},
		migrations.Migration034{},
	}
)

// MigrateUp looks at the currently active migration version
// and will migrate all the way up (applying all up migrations).
func MigrateUp(repoPath, dbPassword string, testnet bool) error {
	version, err := ioutil.ReadFile(path.Join(repoPath, "repover"))
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if err != nil && os.IsNotExist(err) {
		log.Noticef("missing repo version file, migrating from 0")
		version = []byte("0")
	}
	v, err := strconv.Atoi(strings.Trim(string(version), "\n"))
	if err != nil {
		return err
	}
	if v > len(Migrations) {
		log.Errorf("binary can migrate schemas up to version %03d but this schema is already at %03d", len(Migrations), v)
		return ErrUnknownSchema
	}
	x := v
	for _, m := range Migrations[v:] {
		log.Noticef("running migration %03d changing schema to version %03d...\n", x, x+1)
		err := m.Up(repoPath, dbPassword, testnet)
		if err != nil {
			log.Error(err)
			return err
		}
		x++
	}
	return nil
}
