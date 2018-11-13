package datastore_test

import (
	"testing"

	dstore "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	dstest "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/test"
)

func TestMapDatastore(t *testing.T) {
	ds := dstore.NewMapDatastore()
	dstest.SubtestAll(t, ds)
}

func TestNullDatastore(t *testing.T) {
	ds := dstore.NewNullDatastore()
	// The only test that passes. Nothing should be found.
	dstest.SubtestNotFounds(t, ds)
}
