package ipfs

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/ipfs/go-ipfs/core/mock"
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
	n, err := coremock.NewMockNode()
	if err != nil {
		t.Error(err)
	}
	hash, err := AddFile(n, path.Join("./", "root", "test"))
	if err != nil {
		t.Error(err)
	}
	if hash != "Qmf412jQZiuVUtdgnB36FXFX7xg5V6KEbSJ4dpQuhkLyfD" {
		t.Error("Ipfs add file failed")
	}
}

func TestAddDirectory(t *testing.T) {
	n, err := coremock.NewMockNode()
	if err != nil {
		t.Error(err)
	}
	root, err := AddDirectory(n, path.Join("./", "root"))
	if err != nil {
		t.Error(err)
	}
	if root != "QmbuHqv8yQDwSsLvK4wGEBBXAYiqzXn23yqU9rh1tYwJSb" {
		t.Error("Ipfs add directory failed")
	}
}
