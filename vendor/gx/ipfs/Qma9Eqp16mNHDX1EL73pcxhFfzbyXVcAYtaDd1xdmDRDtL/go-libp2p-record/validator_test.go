package record

import (
	"encoding/base64"
	"strings"
	"testing"

	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
)

var OffensiveKey = "CAASXjBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQDjXAQQMal4SB2tSnX6NJIPmC69/BT8A8jc7/gDUZNkEhdhYHvc7k7S4vntV/c92nJGxNdop9fKJyevuNMuXhhHAgMBAAE="

var badPaths = []string{
	"foo/bar/baz",
	"//foo/bar/baz",
	"/ns",
	"ns",
	"ns/",
	"",
	"//",
	"/",
	"////",
}

func TestSplitPath(t *testing.T) {
	ns, key, err := SplitKey("/foo/bar/baz")
	if err != nil {
		t.Fatal(err)
	}
	if ns != "foo" {
		t.Errorf("wrong namespace: %s", ns)
	}
	if key != "bar/baz" {
		t.Errorf("wrong key: %s", key)
	}

	ns, key, err = SplitKey("/foo/bar")
	if err != nil {
		t.Fatal(err)
	}
	if ns != "foo" {
		t.Errorf("wrong namespace: %s", ns)
	}
	if key != "bar" {
		t.Errorf("wrong key: %s", key)
	}

	for _, badP := range badPaths {
		_, _, err := SplitKey(badP)
		if err == nil {
			t.Errorf("expected error for bad path: %s", badP)
		}
	}
}

func TestBadRecords(t *testing.T) {
	v := NamespacedValidator{
		"pk": PublicKeyValidator{},
	}

	sr := u.NewSeededRand(15) // generate deterministic keypair
	_, pubk, err := ci.GenerateKeyPairWithReader(ci.RSA, 1024, sr)
	if err != nil {
		t.Fatal(err)
	}

	pkb, err := pubk.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	for _, badP := range badPaths {
		if v.Validate(badP, pkb) == nil {
			t.Errorf("expected error for path: %s", badP)
		}
	}

	// Test missing namespace
	if v.Validate("/missing/ns", pkb) == nil {
		t.Error("expected error for missing namespace 'missing'")
	}

	// Test valid namespace
	pkh := u.Hash(pkb)
	k := "/pk/" + string(pkh)
	err = v.Validate(k, pkb)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidatePublicKey(t *testing.T) {

	var pkv PublicKeyValidator

	pkb, err := base64.StdEncoding.DecodeString(OffensiveKey)
	if err != nil {
		t.Fatal(err)
	}

	pubk, err := ci.UnmarshalPublicKey(pkb)
	if err != nil {
		t.Fatal(err)
	}

	pkb2, err := pubk.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	pkh := u.Hash(pkb2)
	k := "/pk/" + string(pkh)

	// Good public key should pass
	if err := pkv.Validate(k, pkb); err != nil {
		t.Fatal(err)
	}

	// Bad key format should fail
	var badf = "/aa/" + string(pkh)
	if err := pkv.Validate(badf, pkb); err == nil {
		t.Fatal("Failed to detect bad prefix")
	}

	// Bad key hash should fail
	var badk = "/pk/" + strings.Repeat("A", len(pkh))
	if err := pkv.Validate(badk, pkb); err == nil {
		t.Fatal("Failed to detect bad public key hash")
	}

	// Bad public key should fail
	pkb[0] = 'A'
	if err := pkv.Validate(k, pkb); err == nil {
		t.Fatal("Failed to detect bad public key data")
	}
}

func TestBestRecord(t *testing.T) {
	sel := NamespacedValidator{
		"pk": PublicKeyValidator{},
	}

	i, err := sel.Select("/pk/thing", [][]byte{[]byte("first"), []byte("second")})
	if err != nil {
		t.Fatal(err)
	}
	if i != 0 {
		t.Error("expected to select first record")
	}

	_, err = sel.Select("/pk/thing", nil)
	if err == nil {
		t.Fatal("expected error for no records")
	}

	_, err = sel.Select("/other/thing", [][]byte{[]byte("first"), []byte("second")})
	if err == nil {
		t.Fatal("expected error for unregistered ns")
	}

	_, err = sel.Select("bad", [][]byte{[]byte("first"), []byte("second")})
	if err == nil {
		t.Fatal("expected error for bad key")
	}
}
