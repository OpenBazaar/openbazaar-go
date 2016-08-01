package db

import (
	"database/sql"
	"sync"
)

type InventoryDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (i *InventoryDB) Put(slug string, count int) error {
	i.lock.Lock()
	defer i.lock.Unlock()
	tx, _ := i.db.Begin()
	stmt, _ := tx.Prepare("insert or replace into inventory(slug, count) values(?,?)")
	defer stmt.Close()
	_, err := stmt.Exec(slug, count)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (i *InventoryDB) Get(slug string) (int, error) {
	i.lock.Lock()
	defer i.lock.Unlock()
	stmt, err := i.db.Prepare("select count from inventory where slug=?")
	defer stmt.Close()
	var count int
	err = stmt.QueryRow(slug).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (i *InventoryDB) Delete(slug string) error {
	i.lock.Lock()
	defer i.lock.Unlock()
	_, err := i.db.Exec("delete from inventory where slug=?", slug)
	return err
}
