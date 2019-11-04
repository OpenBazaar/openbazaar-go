package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"sync"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

type InventoryDB struct {
	modelStore
}

func NewInventoryStore(db *sql.DB, lock *sync.Mutex) repo.InventoryStore {
	return &InventoryDB{modelStore{db, lock}}
}

func (i *InventoryDB) Put(slug string, variantIndex int, count *big.Int) error {
	i.lock.Lock()
	defer i.lock.Unlock()

	id := sha256.Sum256([]byte(slug + strconv.Itoa(variantIndex)))

	stmt, err := i.PrepareQuery("insert or replace into inventory(invID, slug, variantIndex, count) values(?,?,?,?)")
	if err != nil {
		return fmt.Errorf("prepare inventory sql: %s", err.Error())
	}
	defer stmt.Close()
	_, err = stmt.Exec(hex.EncodeToString(id[:]), slug, variantIndex, count.String())
	if err != nil {
		return fmt.Errorf("update inventory: %s", err.Error())
	}
	return nil
}

func (i *InventoryDB) GetSpecific(slug string, variantIndex int) (*big.Int, error) {
	i.lock.Lock()
	defer i.lock.Unlock()
	stmt, err := i.db.Prepare("select count from inventory where slug=? and variantIndex=?")
	if err != nil {
		return big.NewInt(0), err
	}
	defer stmt.Close()
	var countStr string
	err = stmt.QueryRow(slug, variantIndex).Scan(&countStr)
	if err != nil {
		return big.NewInt(0), err
	}
	count, ok := new(big.Int).SetString(countStr, 10)
	if !ok {
		return nil, errors.New("error parsing count")
	}
	return count, nil
}

func (i *InventoryDB) Get(slug string) (map[int]*big.Int, error) {
	i.lock.Lock()
	defer i.lock.Unlock()
	ret := make(map[int]*big.Int)
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
		var countStr string
		var variantIndex int
		err = rows.Scan(&slug, &variantIndex, &countStr)
		if err != nil {
			log.Errorf("scanning inventory for (%s): %s", slug, err.Error())
		}
		count, ok := new(big.Int).SetString(countStr, 10)
		if !ok {
			log.Errorf("scanning inventory for (%s): error parsing count", slug)
			count = big.NewInt(0)
		}
		ret[variantIndex] = count
	}
	if err := rows.Err(); err != nil {
		log.Errorf("scanning inventory for (%s): %s", slug, err.Error())
	}
	return ret, nil
}

func (i *InventoryDB) GetAll() (map[string]map[int]*big.Int, error) {
	i.lock.Lock()
	defer i.lock.Unlock()

	ret := make(map[string]map[int]*big.Int)
	stm := "select slug, variantIndex, count from inventory"
	rows, err := i.db.Query(stm)
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var slug string
		var countStr string
		var variantIndex int
		err = rows.Scan(&slug, &variantIndex, &countStr)
		if err != nil {
			log.Error(err)
		}
		count, ok := new(big.Int).SetString(countStr, 10)
		if !ok {
			log.Errorf("scanning inventory for (%s): error parsing count", slug)
			count = big.NewInt(0)
		}

		m, ok := ret[slug]
		if !ok {
			r := make(map[int]*big.Int)
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
