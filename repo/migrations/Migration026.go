package migrations

import (
	"fmt"
	"strings"

	"github.com/ipfs/go-ipfs/repo/fsrepo"
	ds "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	dsq "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore/query"
	ipnspb "gx/ipfs/QmUwMnKKjH3JwGKNVZ3TcP37W93xzqNA4ECFFiMo6sXkkc/go-ipns/pb"
	"gx/ipfs/QmddjPSGZb3ieihSseFeCfVRpZzcqczPNsD2DvarSwnjJB/gogo-protobuf/proto"
)

var cleanIPNSMigrationNumber = 26 // should match name

type (
	cleanIPNSRecordsFromDatastore struct{}
	Migration026                  struct {
		cleanIPNSRecordsFromDatastore
	}
)

// Should we ever update these packages (which functionally changes their
// behavior) the migrations should be made into a no-op.
func (cleanIPNSRecordsFromDatastore) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	r, err := fsrepo.Open(repoPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			log.Errorf("closing repo: %s", err.Error())
		}
	}()

	resultCh, err := r.Datastore().Query(dsq.Query{Prefix: "/ipns/"})
	if err != nil {
		return err
	}
	defer func() {
		if err := resultCh.Close(); err != nil {
			log.Errorf("closing result channel: %s", err.Error())
		}
	}()

	results, err := resultCh.Rest()
	if err != nil {
		return err
	}

	log.Debugf("found %d IPNS records to cull...", len(results))
	for _, rawResult := range results {
		if strings.HasPrefix(rawResult.Key, "/ipns/persistentcache") {
			continue
		}
		rec := new(ipnspb.IpnsEntry)
		if err = proto.Unmarshal(rawResult.Value, rec); err != nil {
			log.Warningf("failed unmarshaling record (%s): %s", rawResult.Key, err.Error())
			if err := r.Datastore().Delete(ds.NewKey(rawResult.Key)); err != nil {
				log.Errorf("failed dropping cached record (%s): %s", rawResult.Key, err.Error())
			}
		}
	}

	if err := writeRepoVer(repoPath, cleanIPNSMigrationNumber+1); err != nil {
		return fmt.Errorf("updating repover to %d: %s", cleanIPNSMigrationNumber+1, err.Error())
	}
	return nil
}

func (cleanIPNSRecordsFromDatastore) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	if err := writeRepoVer(repoPath, cleanIPNSMigrationNumber); err != nil {
		return fmt.Errorf("updating repover to %d: %s", cleanIPNSMigrationNumber, err.Error())
	}
	return nil
}
