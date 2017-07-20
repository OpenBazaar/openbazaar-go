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
	if hash != "zb2rhj7crUKTQYRGCRATFaQ6YFLTde2YzdqbbhAASkL9uRDXn" {
		t.Error("Ipfs add file failed")
	}
}

func TestAddDirectory(t *testing.T) {
	ctx, err := MockCmdsCtx()
	if err != nil {
		t.Error(err)
	}
	root, err := AddDirectory(ctx, path.Join("./", "root"))
	if err != nil {
		t.Error(err)
	}
	if root != "zdj7WgdBhLbZ9f1Z8G3PobEHYk6ArexXBTWTjSCPv97oC4G1U" {
		t.Error("Ipfs add directory failed")
	}
}
