package core

import "testing"

func TestIpnsAPIPathTransform(t *testing.T) {
	peerID := "QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7"
	expected := "https://gateway.ob1.io/ob/ipns/" + peerID

	testVectors := []string{
		"https://gateway.ob1.io",
		"https://gateway.ob1.io/",
	}

	for i, v := range testVectors {
		pth := ipnsAPIPathTransform(v, peerID)
		if pth != expected {
			t.Errorf("IpnsAPIPathTransform test %d failed. Got %s, expected %s", i, pth, expected)
		}
	}
}
