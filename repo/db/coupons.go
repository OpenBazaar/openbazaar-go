package db

import (
	"database/sql"
	"sync"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

type CouponDB struct {
	db   *sql.DB
	lock *sync.Mutex
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
			tx.Rollback()
			return err
		}
	}
	tx.Commit()
	return nil
}

func (c *CouponDB) Get(slug string) ([]repo.Coupon, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	var stm string
	stm = "select slug, code, hash from coupons where slug='" + slug + "';"
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
		rows.Scan(&slug, &code, &hash)
		ret = append(ret, repo.Coupon{slug, code, hash})
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
