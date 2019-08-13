package repo_test

import (
	"bytes"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestPeerInfoFromProtobuf(t *testing.T) {
	var (
		validFixture = factory.MustNewValidPeerIDProtobuf()
		subject, err = repo.NewPeerInfoFromProtobuf(validFixture)
	)
	if err != nil {
		t.Fatal(err)
	}

	if hash, err := subject.Hash(); err == nil && hash != validFixture.PeerID {
		t.Errorf("expected Hash() to be (%s), but was (%s)", validFixture.PeerID, hash)
		//if err != nil {
		//	t.Logf("error from Hash(): %s", err)
		//}
	}

	if subject.Handle() != validFixture.Handle {
		t.Errorf("expected Handle() to be (%s), but was (%s)", validFixture.Handle, subject.Handle())
	}

	if !bytes.Equal(subject.BitcoinSignature(), validFixture.BitcoinSig) {
		t.Errorf("expected BitcoinSignature() to be (%s), but was (%s)", validFixture.BitcoinSig, subject.BitcoinSignature())
	}

	if !bytes.Equal(subject.BitcoinKey(), validFixture.Pubkeys.Bitcoin) {
		t.Errorf("expected BitcoinKey() to be (%s), but was (%s)", validFixture.Pubkeys.Bitcoin, subject.BitcoinKey())
	}

	if !bytes.Equal(subject.IdentityKeyBytes(), validFixture.Pubkeys.Identity) {
		t.Errorf("expected IdentityKey() to be (%s), but was (%s)", validFixture.Pubkeys.Identity, subject.IdentityKeyBytes())
	}

	newProto := subject.Protobuf()
	duplicateSubject, err := repo.NewPeerInfoFromProtobuf(newProto)
	if err != nil {
		t.Fatal(err)
	}
	if !subject.Equal(duplicateSubject) {
		t.Error("expected Protobuf() to produce recipricol of NewPeerInfoFromProtobuf, but did not")
		t.Logf("\texpected: %+v\n", subject)
		t.Logf("\tactual: %+v\n", duplicateSubject)
	}
}

func TestPeerInfoFromIdentityKey(t *testing.T) {
	var (
		testPeer      = factory.MustNewValidPeerInfo()
		testHash, err = testPeer.Hash()
		subject       = repo.NewPeerInfoFromIdentityKey(testPeer.IdentityKeyBytes())
	)
	if err != nil {
		t.Fatal(err)
	}

	var (
		actual = repo.NewPeerInfoFromIdentityKey(testPeer.IdentityKeyBytes())
	)

	if !bytes.Equal(actual.IdentityKeyBytes(), testPeer.IdentityKeyBytes()) {
		t.Error("expected peerInfo to have same internal key, but did not")
	}

	if hash, err := subject.Hash(); err == nil && hash != testHash {
		t.Error("expected derived hash to match test peer, but did not")
		//if err != nil {
		//	t.Logf("error from Hash(): %s", err)
		//}
	}
}

func TestPeerInfoValid(t *testing.T) {
	// MustNewValidPeerInfo forces a panic in the event internal logic has changed
	factory.MustNewValidPeerInfo()

	var pp = factory.MustNewValidPeerIDProtobuf()
	pp.PeerID = "invalidstring"
	p, err := repo.NewPeerInfoFromProtobuf(pp)
	if err != nil {
		t.Fatal(err)
	}
	v, errs := p.Valid()
	if v == true {
		t.Fatal("expected an invalid peer id to make the object invalid")
	}

	// Check for ErrInvalidInlinePeerID
	var foundErr bool
	for _, e := range errs {
		if e == repo.ErrInvalidInlinePeerID {
			foundErr = true
		}
	}
	if !foundErr {
		t.Error("expected to find ErrInvalidInlinePeerID in errors, but did not")
		t.Logf("errors: %+v", errs)
	}
}

func TestNilPeerInfo(t *testing.T) {
	var nilPeer *repo.PeerInfo
	if nilPeer.Equal(nilPeer) {
		t.Errorf("expected nil *PeerInfo.Equal() to be false, but was not")
	}

	if isValid, _ := nilPeer.Valid(); isValid {
		t.Errorf("expected nil *PeerInfo to be invalid, but was valid")
	}

	h, err := nilPeer.Hash()
	if h != "" {
		t.Errorf("expected nil *PeerInfo.Hash() to be empty, but was not")
	}
	if err != repo.ErrPeerInfoIsNil {
		t.Errorf("expected nil *PeerInfo.Hash() respond with the appropriate error, but did not")
	}
}
