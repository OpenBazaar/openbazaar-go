package bench_test

import (
	"fmt"
	"os"
	"testing"

	"context"
	"gx/ipfs/QmSJBsmLP1XMjv8hxYg2rUMdPDB7YUpyBo9idjrJ6Cmq6F/fuse"
	"gx/ipfs/QmSJBsmLP1XMjv8hxYg2rUMdPDB7YUpyBo9idjrJ6Cmq6F/fuse/fs"
	"gx/ipfs/QmSJBsmLP1XMjv8hxYg2rUMdPDB7YUpyBo9idjrJ6Cmq6F/fuse/fs/fstestutil"
)

type dummyFile struct {
	fstestutil.File
}

type benchCreateDir struct {
	fstestutil.Dir
}

var _ fs.NodeCreater = (*benchCreateDir)(nil)

func (f *benchCreateDir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	child := &dummyFile{}
	return child, child, nil
}

func BenchmarkCreate(b *testing.B) {
	f := &benchCreateDir{}
	mnt, err := fstestutil.MountedT(b, fstestutil.SimpleFS{f}, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer mnt.Close()

	// prepare file names to decrease test overhead
	names := make([]string, 0, b.N)
	for i := 0; i < b.N; i++ {
		// zero-padded so cost stays the same on every iteration
		names = append(names, mnt.Dir+"/"+fmt.Sprintf("%08x", i))
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		f, err := os.Create(names[i])
		if err != nil {
			b.Fatalf("WriteFile: %v", err)
		}
		f.Close()
	}

	b.StopTimer()
}
