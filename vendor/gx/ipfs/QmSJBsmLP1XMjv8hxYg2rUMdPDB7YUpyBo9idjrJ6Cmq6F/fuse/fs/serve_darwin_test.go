package fs_test

import (
	"testing"

	"gx/ipfs/QmSJBsmLP1XMjv8hxYg2rUMdPDB7YUpyBo9idjrJ6Cmq6F/fuse/fs/fstestutil"
	"gx/ipfs/QmVGjyM9i2msKvLXwh9VosCTgP4mL91kC7hDmqnwTTx6Hu/sys/unix"
)

type exchangeData struct {
	fstestutil.File
	// this struct cannot be zero size or multiple instances may look identical
	_ int
}

func TestExchangeDataNotSupported(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{
		"one": &exchangeData{},
		"two": &exchangeData{},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	if err := unix.Exchangedata(mnt.Dir+"/one", mnt.Dir+"/two", 0); err != unix.ENOTSUP {
		t.Fatalf("expected ENOTSUP from exchangedata: %v", err)
	}
}
