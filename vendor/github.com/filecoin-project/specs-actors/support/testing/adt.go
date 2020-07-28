package testing

import (
	"testing"

	"github.com/ipfs/go-cid"
)

type rooter interface {
	Root() (cid.Cid, error)
}

func MustRoot(t testing.TB, r rooter) cid.Cid {
	t.Helper()
	c, err := r.Root()
	if err != nil {
		t.Fatal(err)
	}
	return c
}
