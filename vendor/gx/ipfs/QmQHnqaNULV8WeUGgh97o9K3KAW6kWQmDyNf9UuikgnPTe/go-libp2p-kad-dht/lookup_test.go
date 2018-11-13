package dht

import (
	"testing"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
)

func TestLoggableKey(t *testing.T) {
	c, err := cid.Decode("QmfUvYQhL2GinafMbPDYz7VFoZv4iiuLuR33aRsPurXGag")
	if err != nil {
		t.Fatal(err)
	}

	k, err := tryFormatLoggableKey("/proto/" + string(c.Bytes()))
	if err != nil {
		t.Errorf("failed to format key 1: %s", err)
	}
	if k != "/proto/"+c.String() {
		t.Error("expected path to be preserved as a loggable key")
	}

	k, err = tryFormatLoggableKey(string(c.Bytes()))
	if err != nil {
		t.Errorf("failed to format key 2: %s", err)
	}
	if k != "/provider/"+c.String() {
		t.Error("expected cid to be formatted as a loggable key")
	}

	for _, s := range []string{"bla bla", "/bla", "/bla/asdf", ""} {
		if _, err := tryFormatLoggableKey(s); err == nil {
			t.Errorf("expected to fail formatting: %s", s)
		}
	}
}
