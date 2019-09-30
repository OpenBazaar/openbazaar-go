package db

import (
	"database/sql"
	"sync"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

type CouponDB struct {
	modelStore
}

func NewCouponStore(db *sql.DB, lock *sync.Mutex) repo.CouponStore {
	return &CouponDB{modelStore{db, lock}}
}

func (c *CouponDB) Put(coupons []repo.Coupon) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	tx, _ := c.db.Begin()
	for _, coupon := range coupons {
		stmt, _ := tx.Prepare("insert or replace into coupons(slug, code, hash) values(?,?,?)")
		defer stmt.Close()
		_, err := stmt.Exec(coupon.Slug, coupon.Code, coupon.Hash)
		if err != nil {
			err = tx.Rollback()
			if err != nil {
				log.Error(err)
			}
			return err
		}
	}
	err := tx.Commit()
	if err != nil {
		log.Error(err)
	}
	return nil
}

func (c *CouponDB) Get(slug string) ([]repo.Coupon, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	stm := "select slug, code, hash from coupons where slug='" + slug + "';"
	rows, err := c.db.Query(stm)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	defer rows.Close()
	var ret []repo.Coupon
	for rows.Next() {
		var slug string
		var code string
		var hash string
		err = rows.Scan(&slug, &code, &hash)
		if err != nil {
			log.Error(err)
		}
		ret = append(ret, repo.Coupon{Slug: slug, Code: code, Hash: hash})
	}
	return ret, nil
}

func (c *CouponDB) Delete(slug string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, err := c.db.Exec("delete from coupons where slug=?", slug)
	if err != nil {
		return err
	}
	return nil
}
