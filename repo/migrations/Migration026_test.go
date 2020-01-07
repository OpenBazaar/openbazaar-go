package migrations

import (
	"bytes"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/ipfs/go-ipfs/repo/fsrepo"

	ds "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
)

func TestCleanIPNSRecordsMigration(t *testing.T) {
	var (
		basePath             = schema.GenerateTempPath()
		ipnsKey              = "/ipns/shoulddelete"
		ipnsFalsePositiveKey = "/ipns/persistentcache/shouldNOTdelete"
		otherKey             = "/ipfs/shouldNOTdelete"
		migration            = cleanIPNSRecordsFromDatastore{}

		testRepoPath, err = schema.OpenbazaarPathTransform(basePath, true)
	)
	if err != nil {
		t.Fatal(err)
	}

	appSchema, err := schema.NewCustomSchemaManager(schema.SchemaContext{DataPath: testRepoPath, TestModeEnabled: true})
	if err != nil {
		t.Fatal(err)
	}

	if err = appSchema.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer appSchema.DestroySchemaDirectories()

	if err := fsrepo.Init(appSchema.DataPath(), schema.MustDefaultConfig()); err != nil {
		t.Fatal(err)
	}

	r, err := fsrepo.Open(testRepoPath)
	if err != nil {
		t.Fatalf("opening repo: %s", err.Error())
	}

	err = r.Datastore().Put(ds.NewKey(ipnsKey), []byte("randomdata"))
	if err != nil {
		t.Fatal("unable to put ipns record")
	}
	err = r.Datastore().Put(ds.NewKey(ipnsFalsePositiveKey), []byte("randomdata"))
	if err != nil {
		t.Fatal("unable to put other record")
	}
	err = r.Datastore().Put(ds.NewKey(otherKey), []byte("randomdata"))
	if err != nil {
		t.Fatal("unable to put other record")
	}

	// run migration up
	if err := migration.Up(appSchema.DataPath(), "", true); err != nil {
		t.Fatal(err)
	}

	// validate state
	if _, err := r.Datastore().Get(ds.NewKey(ipnsKey)); err != ds.ErrNotFound {
		t.Errorf("expected the IPNS record to be removed, but was not")
	}
	if val, err := r.Datastore().Get(ds.NewKey(ipnsFalsePositiveKey)); err != nil {
		t.Errorf("expected the false-positive record to be present, but was not")
	} else {
		if !bytes.Equal([]byte("randomdata"), val) {
			t.Errorf("expected the false-positive record data to be intact, but was not")
		}
	}
	if val, err := r.Datastore().Get(ds.NewKey(otherKey)); err != nil {
		t.Errorf("expected the other record to be present, but was not")
	} else {
		if !bytes.Equal([]byte("randomdata"), val) {
			t.Errorf("expected the other record data to be intact, but was not")
		}
	}

	if err = appSchema.VerifySchemaVersion("27"); err != nil {
		t.Fatal(err)
	}

	// run migration down
	if err := migration.Down(appSchema.DataPath(), "", true); err != nil {
		t.Fatal(err)
	}

	// validate state
	if err = appSchema.VerifySchemaVersion("26"); err != nil {
		t.Fatal(err)
	}
}
