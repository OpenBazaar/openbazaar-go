package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strconv"
	"sync"
)

type InventoryDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (i *InventoryDB) Put(slug string, variantIndex int, count int) error {
	i.lock.Lock()
	defer i.lock.Unlock()

	id := sha256.Sum256([]byte(slug + strconv.Itoa(variantIndex)))

	tx, _ := i.db.Begin()
	stmt, err := tx.Prepare("insert or replace into inventory(invID, slug, variantIndex, count) values(?,?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(hex.EncodeToString(id[:]), slug, variantIndex, count)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (i *InventoryDB) GetSpecific(slug string, variantIndex int) (int, error) {
	i.lock.Lock()
	defer i.lock.Unlock()
	stmt, err := i.db.Prepare("select count from inventory where slug=? and variantIndex=?")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	var count int
	err = stmt.QueryRow(slug, variantIndex).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (i *InventoryDB) Get(slug string) (map[int]int, error) {
	i.lock.Lock()
	defer i.lock.Unlock()
	ret := make(map[int]int)
	stmt, err := i.db.Prepare("select slug, variantIndex, count from inventory where slug=?")
	if err != nil {
		return ret, err
	}
	defer stmt.Close()
	rows, err := stmt.Query(slug)
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var slug string
		var count int
		var variantIndex int
		rows.Scan(&slug, &variantIndex, &count)
		ret[variantIndex] = count
	}
	return ret, nil
}

func (i *InventoryDB) GetAll() (map[string]map[int]int, error) {
	i.lock.Lock()
	defer i.lock.Unlock()

	ret := make(map[string]map[int]int)
	stm := "select slug, variantIndex, count from inventory"
	rows, err := i.db.Query(stm)
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var slug string
		var count int
		var variantIndex int
		rows.Scan(&slug, &variantIndex, &count)
		m, ok := ret[slug]
		if !ok {
			r := make(map[int]int)
			r[variantIndex] = count
			ret[slug] = r
		} else {
			m[variantIndex] = count
		}
	}
	return ret, nil
}

func (i *InventoryDB) Delete(slug string, variantIndex int) error {
	i.lock.Lock()
	defer i.lock.Unlock()
	_, err := i.db.Exec("delete from inventory where slug=? and variantIndex=?", slug, variantIndex)
	return err
}

func (i *InventoryDB) DeleteAll(slug string) error {
	i.lock.Lock()
	defer i.lock.Unlock()
	_, err := i.db.Exec("delete from inventory where slug=?", slug)
	return err
}
