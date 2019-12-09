package repo_test

import (
	"bytes"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestPeerInfoFromProtobuf(t *testing.T) {
	var (
		validFixture = factory.MustNewPeerIDProtobuf()
		subject      = repo.NewPeerInfoFromProtobuf(validFixture)
	)

	if hash, err := subject.Hash(); err == nil && hash != validFixture.PeerID {
		t.Errorf("expected Hash() to be (%s), but was (%s)", validFixture.PeerID, hash)
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
	duplicateSubject := repo.NewPeerInfoFromProtobuf(newProto)

	if !subject.Equal(duplicateSubject) {
		t.Error("expected Protobuf() to produce recipricol of NewPeerInfoFromProtobuf, but did not")
		t.Logf("\texpected: %+v\n", subject)
		t.Logf("\tactual: %+v\n", duplicateSubject)
	}
}

func TestPeerInfoValid(t *testing.T) {
	// MustNewValidPeerInfo forces a panic in the event internal logic has changed
	factory.MustNewValidPeerInfo()

	var pp = factory.MustNewPeerIDProtobuf()
	pp.PeerID = "invalidstring"
	p := repo.NewPeerInfoFromProtobuf(pp)

	err := p.Valid()
	if err == nil {
		t.Fatal("expected peer info to not be valid")
	}
}

func TestNilPeerInfo(t *testing.T) {
	var nilPeer *repo.PeerInfo
	if nilPeer.Equal(nilPeer) {
		t.Errorf("expected nil *PeerInfo.Equal() to be false, but was not")
	}

	if err := nilPeer.Valid(); err == nil {
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
