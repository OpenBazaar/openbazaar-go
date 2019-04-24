package ipfs

import (
	"os"
	"path"
	"testing"

	"github.com/ipfs/go-ipfs/core/mock"
)

func TestUnPinDir(t *testing.T) {
	n, err := coremock.NewMockNode()
	if err != nil {
		t.Error(err)
	}

	root, err := AddDirectory(n, path.Join(os.TempDir(), "root"))
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
