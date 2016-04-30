package db

import (
	"sync"
	"database/sql"
)

type FollowerDB struct {
	db *sql.DB
	lock sync.Mutex
}

func (f *FollowerDB) Put(follower string) error {
	return nil
}

func (f *FollowerDB) Get(startIndex uint, numToReturn uint) []string {
	return nil
}

func (f *FollowerDB) Delete(follower string) error {
	return nil
}