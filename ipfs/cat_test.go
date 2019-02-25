package ipfs

import (
	"testing"
	"time"

	"github.com/ipfs/go-ipfs/core/mock"
)

func TestCidCompatibility(t *testing.T) {
	tests := []struct {
		path  string
		valid bool
	}{
		{"/ipfs/zb2rhfXc9CE96apvjFYZaSAEg25ffbgcbSW4hg2BX8ucW5LRE", true},
		{"/ipfs/QmTHCE9EEcDi9mZqdp2JF61n4fkYRjSJbRxYwtoY7ofjJp", true},
		{"/ipfs/asdfasdfadsf", false},
	}

	n, err := coremock.NewMockNode()
	if err != nil {
		t.Error(err)
	}
	invalidCidErrorString := "selected encoding not supported"
	for i, test := range tests {
		_, err = Cat(n, test.path, time.Millisecond)
		if !test.valid && err.Error() != invalidCidErrorString || test.valid && err.Error() == invalidCidErrorString {
			t.Errorf("TestCidCompatibility failed to correctly parse test %d", i)
		}
	}
}
