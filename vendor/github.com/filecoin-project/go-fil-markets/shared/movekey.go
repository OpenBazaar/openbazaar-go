package shared

import "github.com/ipfs/go-datastore"

// MoveKey moves a key in a data store
func MoveKey(ds datastore.Datastore, old string, new string) error {
	oldKey := datastore.NewKey(old)
	newKey := datastore.NewKey(new)
	has, err := ds.Has(oldKey)
	if err != nil {
		return err
	}
	if !has {
		return nil
	}
	value, err := ds.Get(oldKey)
	if err != nil {
		return err
	}
	err = ds.Put(newKey, value)
	if err != nil {
		return err
	}
	err = ds.Delete(oldKey)
	if err != nil {
		return err
	}
	return nil
}
