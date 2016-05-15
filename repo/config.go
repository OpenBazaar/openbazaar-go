package repo

import (
	"github.com/ipfs/go-ipfs/repo"
)

// TODO: functions for fetching custom fields from config file

func extendConfigFile(r repo.Repo, key string, value interface{}) error {
	if err := r.SetConfigKey(key, value); err != nil {
		return err
	}
	if err := r.Close(); err != nil {
		return err
	}
	return nil
}