package ipfs

import (
	"path"
	"testing"
)

func TestUnPinDir(t *testing.T) {
	ctx, err := MockCmdsCtx()
	if err != nil {
		t.Error(err)
	}
	root, err := AddDirectory(ctx, path.Join("./", "root"))
	if err != nil {
		t.Error(err)
	}
	err = UnPinDir(ctx, root)
	if err != nil {
		t.Error(err)
	}
	err = UnPinDir(ctx, "fasdfasdf")
	if err == nil {
		t.Error("Should have through error unpinning known directory")
	}
}
