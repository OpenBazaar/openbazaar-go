package autobatch

import (
	"bytes"
	"fmt"
	"testing"

	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	dstest "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/test"
)

func TestAutobatch(t *testing.T) {
	dstest.SubtestAll(t, NewAutoBatching(ds.NewMapDatastore(), 16))
}

func TestFlushing(t *testing.T) {
	child := ds.NewMapDatastore()
	d := NewAutoBatching(child, 16)

	var keys []ds.Key
	for i := 0; i < 16; i++ {
		keys = append(keys, ds.NewKey(fmt.Sprintf("test%d", i)))
	}
	v := []byte("hello world")

	for _, k := range keys {
		err := d.Put(k, v)
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err := child.Get(keys[0])
	if err != ds.ErrNotFound {
		t.Fatal("shouldnt have found value")
	}

	err = d.Put(ds.NewKey("test16"), v)
	if err != nil {
		t.Fatal(err)
	}

	// should be flushed now, try to get keys from child datastore
	for _, k := range keys {
		val, err := child.Get(k)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(val, v) {
			t.Fatal("wrong value")
		}
	}
}
