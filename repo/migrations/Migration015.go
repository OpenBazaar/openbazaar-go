package migrations

import (
	"github.com/OpenBazaar/openbazaar-go/schema"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
)

// Migration015 is part of the multiwallet refactor. We are moving the data directory to a
// single canonical location ― .openbazaar. Currently it may reside in .openbazaar2.0,
// .openbazaar2.0-bitcoincash or .openbazaar2.0-zcash. If .openbazaar exists we will
// move it to .openbazaar.old and replace it with our data folder. If .openbazaar exists
// and contains a fully migrated data directory then we will instead migrate in place.
type Migration015 struct {
	ctx schema.SchemaContext
}

func (m Migration015) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	mgr, err := schema.NewCustomSchemaManager(m.ctx)
	if err != nil {
		return err
	}
	// If this is a custom data directory then just migrate in place
	if repoPath != mgr.LegacyDataPath() {
		return writeRepoVer(repoPath, 16)
	}
	newRepoPath := mgr.NewDataPath()
	if _, err := os.Stat(newRepoPath); os.IsNotExist(err) {
		// New repo directory does not exist so we can
		// just copy the current directory over.
		if err := os.Rename(repoPath, newRepoPath); err != nil {
			return err
		}

	} else {
		// The new data directory appears to contain another already migrated
		// instance. In this case we will just migrate in place and not move
		// the directory.
		version, err := ioutil.ReadFile(path.Join(newRepoPath, "repover"))
		if err == nil {
			i, err := strconv.Atoi(strings.Trim(string(version), "\n"))
			if err == nil && i >= 16 {
				return writeRepoVer(repoPath, 16)
			}
		}
		// Something else is already in the new path so let's rename it first
		if err := os.Rename(newRepoPath, newRepoPath+".old"); err != nil {
			return err
		}

		// Now we can move the directory over.
		if err := os.Rename(repoPath, newRepoPath); err != nil {
			return err
		}
	}

	// Bump schema version
	return writeRepoVer(newRepoPath, 16)
}

func (m Migration015) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	mgr, err := schema.NewCustomSchemaManager(m.ctx)
	if err != nil {
		return err
	}
	legacyRepoPath := mgr.LegacyDataPath()

	// If we're already in the legacy repo path then just migrate down in place.
	if repoPath == legacyRepoPath {
		return writeRepoVer(repoPath, 15)
	}
	if _, err := os.Stat(legacyRepoPath); os.IsNotExist(err) {
		// New repo directory does not exist so we can
		// just copy the current directory over.
		if err := os.Rename(repoPath, legacyRepoPath); err != nil {
			return err
		}

	} else {
		// Something is already in the new path so let's rename it first
		if err := os.Rename(legacyRepoPath, legacyRepoPath+".old"); err != nil {
			return err
		}

		// Now we can move the directory over.
		if err := os.Rename(repoPath, legacyRepoPath); err != nil {
			return err
		}
	}

	// Revert schema version
	return writeRepoVer(legacyRepoPath, 15)
}
