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
	err := os.MkdirAll(path.Join(os.TempDir(), "root"), os.ModePerm)
	if err != nil {
		log.Error(err)
	}
	d := []byte("hello world")
	err = ioutil.WriteFile(path.Join(os.TempDir(), "root", "test"), d, os.ModePerm)
	if err != nil {
		log.Error(err)
	}
}

func teardown() {
	os.RemoveAll(path.Join(os.TempDir(), "root"))
}

func TestAddFile(t *testing.T) {
	n, err := coremock.NewMockNode()
	if err != nil {
		t.Error(err)
	}
	hash, err := AddFile(n, path.Join(os.TempDir(), "root", "test"))
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
	root, err := AddDirectory(n, path.Join(os.TempDir(), "root"))
	if err != nil {
		t.Error(err)
	}
	if root != "QmbuHqv8yQDwSsLvK4wGEBBXAYiqzXn23yqU9rh1tYwJSb" {
		t.Error("Ipfs add directory failed")
	}
}
