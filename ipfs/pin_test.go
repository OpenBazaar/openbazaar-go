package ipfs

import (
	"github.com/ipfs/go-ipfs/core/mock"
	"path"
	"testing"
)

func TestUnPinDir(t *testing.T) {
	n, err := coremock.NewMockNode()
	if err != nil {
		t.Error(err)
	}

	root, err := AddDirectory(n, path.Join("./", "root"))
	if err != nil {
		t.Error(err)
	}

	err = UnPinDir(n, root)
	if err != nil {
		t.Error(err)
	}
	err = UnPinDir(n, "fasdfasdf")
	if err == nil {
		t.Error("Shouldn't have thrown an error")
	}
}
