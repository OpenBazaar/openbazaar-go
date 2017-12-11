package db

import (
	"database/sql"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"sync"
	"testing"
)

var coup CouponDB

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	coup = CouponDB{
		db:   conn,
		lock: new(sync.Mutex),
	}
}

func TestPutCoupons(t *testing.T) {
	coupons := []repo.Coupon{
		{"slug", "code1", "hash1"},
		{"slug", "code2", "hash2"},
	}
	err := coup.Put(coupons)
	if err != nil {
		t.Error(err)
	}
	stm := "select slug, code, hash from coupons where slug=slug;"
	rows, err := coup.db.Query(stm)
	if err != nil {
		t.Error(err)
		return
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
	if len(ret) != 2 {
		t.Error("Failed to return correct number of coupons")
	}
	if ret[0].Slug != "slug" || ret[0].Code != "code1" || ret[0].Hash != "hash1" {
		t.Error("Failed to return correct values")
	}
	if ret[1].Slug != "slug" || ret[1].Code != "code2" || ret[1].Hash != "hash2" {
		t.Error("Failed to return correct values")
	}
}

func TestGetCoupons(t *testing.T) {
	coupons := []repo.Coupon{
		{"s", "code1", "hash1"},
		{"s", "code2", "hash2"},
	}
	err := coup.Put(coupons)
	if err != nil {
		t.Error(err)
	}
	ret, err := coup.Get("s")
	if err != nil {
		t.Error(err)
	}
	if len(ret) != 2 {
		t.Error("Failed to return correct number of coupons")
	}
	if ret[0].Slug != "s" || ret[0].Code != "code1" || ret[0].Hash != "hash1" {
		t.Error("Failed to return correct values")
	}
	if ret[1].Slug != "s" || ret[1].Code != "code2" || ret[1].Hash != "hash2" {
		t.Error("Failed to return correct values")
	}
}

func TestDeleteCoupons(t *testing.T) {
	coupons := []repo.Coupon{
		{"slug", "code1", "hash1"},
		{"slug", "code2", "hash2"},
	}
	err := coup.Put(coupons)
	if err != nil {
		t.Error(err)
	}
	err = coup.Delete("slug")
	if err != nil {
		t.Error(err)
	}
	ret, err := coup.Get("slug")
	if err != nil {
		t.Error(err)
	}
	if len(ret) != 0 {
		t.Error("Failed to delete coupons")
	}
}
