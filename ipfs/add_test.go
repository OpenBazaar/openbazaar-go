package ipfs

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func TestMain(m *testing.M) {
	setup()
	retCode := m.Run()
	teardown()
	os.Exit(retCode)
}

func setup() {
	os.MkdirAll(path.Join("./", "root"), os.ModePerm)
	d1 := []byte("hello world")
	ioutil.WriteFile(path.Join("./", "root", "test"), d1, 0644)
}

func teardown() {
	os.RemoveAll(path.Join("./", "root"))
}

func TestAddFile(t *testing.T) {
	ctx, err := MockCmdsCtx()
	if err != nil {
		t.Error(err)
	}
	hash, err := AddFile(ctx, path.Join("./", "root", "test"))
	if err != nil {
		t.Error(err)
	}
	if hash != "Qmf412jQZiuVUtdgnB36FXFX7xg5V6KEbSJ4dpQuhkLyfD" {
		t.Error("Ipfs add file failed")
	}
}

func TestAddDirectory(t *testing.T) {
	ctx, err := MockCmdsCtx()
	if err != nil {
		t.Error(err)
	}
	hash, err := AddDirectory(ctx, path.Join("./", "root"))
	if err != nil {
		t.Error(err)
	}
	if hash != "QmbuHqv8yQDwSsLvK4wGEBBXAYiqzXn23yqU9rh1tYwJSb" {
		t.Error("Ipfs add directory failed")
	}
}
