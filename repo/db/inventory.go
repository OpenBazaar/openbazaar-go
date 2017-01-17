package db

import (
	"database/sql"
	"sync"
)

type InventoryDB struct {
	db   *sql.DB
	lock sync.RWMutex
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

func (i *InventoryDB) GetSpecific(path string) (int, error) {
	i.lock.RLock()
	defer i.lock.RUnlock()
	stmt, err := i.db.Prepare("select count from inventory where slug=?")
	defer stmt.Close()
	var count int
	err = stmt.QueryRow(path).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (i *InventoryDB) Get(slug string) (map[string]int, error) {
	i.lock.RLock()
	defer i.lock.RUnlock()
	ret := make(map[string]int)
	stm := `select * from inventory where slug like "` + slug + `%";`
	rows, err := i.db.Query(stm)
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var slug string
		var count int
		rows.Scan(&slug, &count)
		ret[slug] = count
	}
	return ret, nil
}

func (i *InventoryDB) GetAll() (map[string]int, error) {
	i.lock.RLock()
	defer i.lock.RUnlock()
	ret := make(map[string]int)
	stm := "select slug, count from inventory"
	rows, err := i.db.Query(stm)
	defer rows.Close()
	if err != nil {
		return ret, err
	}
	for rows.Next() {
		var slug string
		var count int
		rows.Scan(&slug, &count)
		ret[slug] = count
	}
	return ret, nil
}

func (i *InventoryDB) Delete(path string) error {
	i.lock.Lock()
	defer i.lock.Unlock()
	_, err := i.db.Exec("delete from inventory where slug=?", path)
	return err
}

func (i *InventoryDB) DeleteAll(slug string) error {
	i.lock.Lock()
	defer i.lock.Unlock()
	stm := `delete from inventory where slug like "` + slug + `%";`
	_, err := i.db.Exec(stm)
	return err
}
