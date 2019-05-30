package repo_test

import (
	"bytes"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func TestPeerIDFromProtobuf(t *testing.T) {
	var subject = factory.NewPeerIDProtobuf()
	subject.PeerID = "peerID"
	subject.Handle = "handle"
	subject.BitcoinSig = []byte("bitcoinsig")
	subject.Pubkeys.Identity = []byte("identitykey")
	subject.Pubkeys.Bitcoin = []byte("bitcoinkey")

	var actual = repo.NewPeerIDFromProtobuf(subject)
	if actual.Hash() != subject.PeerID {
		t.Errorf("expected Hash() to be (%s), but was (%s)", subject.PeerID, actual.Hash())
	}
	if actual.Handle() != subject.Handle {
		t.Errorf("expected Handle() to be (%s), but was (%s)", subject.Handle, actual.Handle())
	}
	if !bytes.Equal(actual.BitcoinSignature(), subject.BitcoinSig) {
		t.Errorf("expected BitcoinSignature() to be (%s), but was (%s)", subject.BitcoinSig, actual.BitcoinSignature())
	}
	if !bytes.Equal(actual.BitcoinKey(), subject.Pubkeys.Bitcoin) {
		t.Errorf("expected BitcoinKey() to be (%s), but was (%s)", subject.Pubkeys.Bitcoin, actual.BitcoinKey())
	}
	if !bytes.Equal(actual.IdentityKey(), subject.Pubkeys.Identity) {
		t.Errorf("expected IdentityKey() to be (%s), but was (%s)", subject.Pubkeys.Identity, actual.IdentityKey())
	}
}
