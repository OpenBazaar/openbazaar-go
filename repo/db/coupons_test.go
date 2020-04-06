package db_test

import (
	"sync"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func buildNewCouponStore() (repo.CouponStore, func(), error) {
	appSchema := schema.MustNewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err := appSchema.BuildSchemaDirectories(); err != nil {
		return nil, nil, err
	}
	if err := appSchema.InitializeDatabase(); err != nil {
		return nil, nil, err
	}
	database, err := appSchema.OpenDatabase()
	if err != nil {
		return nil, nil, err
	}
	return db.NewCouponStore(database, new(sync.Mutex)), appSchema.DestroySchemaDirectories, nil
}

func TestPutCoupons(t *testing.T) {
	var couponDB, teardown, err = buildNewCouponStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	coupons := []repo.Coupon{
		{Slug: "slug", Code: "code1", Hash: "hash1"},
		{Slug: "slug", Code: "code2", Hash: "hash2"},
	}
	err = couponDB.Put(coupons)
	if err != nil {
		t.Error(err)
	}
	stm := "select slug, code, hash from coupons where slug=slug;"
	rows, err := couponDB.PrepareAndExecuteQuery(stm)
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
		err = rows.Scan(&slug, &code, &hash)
		if err != nil {
			t.Log(err)
		}
		ret = append(ret, repo.Coupon{Slug: slug, Code: code, Hash: hash})
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
	var couponDB, teardown, err = buildNewCouponStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	coupons := []repo.Coupon{
		{Slug: "s", Code: "code1", Hash: "hash1"},
		{Slug: "s", Code: "code2", Hash: "hash2"},
	}
	err = couponDB.Put(coupons)
	if err != nil {
		t.Error(err)
	}
	ret, err := couponDB.Get("s")
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
	var couponDB, teardown, err = buildNewCouponStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	coupons := []repo.Coupon{
		{Slug: "slug", Code: "code1", Hash: "hash1"},
		{Slug: "slug", Code: "code2", Hash: "hash2"},
	}
	err = couponDB.Put(coupons)
	if err != nil {
		t.Error(err)
	}
	err = couponDB.Delete("slug")
	if err != nil {
		t.Error(err)
	}
	ret, err := couponDB.Get("slug")
	if err != nil {
		t.Error(err)
	}
	if len(ret) != 0 {
		t.Error("Failed to delete coupons")
	}
}
