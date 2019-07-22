package ipfs

import (
	"github.com/ipfs/go-ipfs/core/mock"
	"github.com/ipfs/go-ipfs/namesys"
	ipath "gx/ipfs/QmQAgv6Gaoe2tQpcabqwKXKChp2MZ7i3UXv9DqTTaxCaTR/go-path"
	ipnspb "gx/ipfs/QmUwMnKKjH3JwGKNVZ3TcP37W93xzqNA4ECFFiMo6sXkkc/go-ipns/pb"
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	"gx/ipfs/QmddjPSGZb3ieihSseFeCfVRpZzcqczPNsD2DvarSwnjJB/gogo-protobuf/proto"
	"testing"
)

// TestDatastoreCaching tests that the data can be inserted and retrieved from the
// database using the IPNS namespace as well as our persistent cache namespace and
// that the two do not conflict with each other.
func TestDatastoreCaching(t *testing.T) {
	n, err := coremock.NewMockNode()
	if err != nil {
		t.Error(err)
	}
	var (
		pth       = "/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h"
		rec       = &ipnspb.IpnsEntry{Value: []byte(pth), Signature: []byte{}}
		peerIDStr = "QmddjPSGZb3ieihSseFeCfVRpZzcqczPNsD2DvarSwnjJB"
	)

	peerID, err := peer.IDB58Decode(peerIDStr)
	if err != nil {
		t.Fatal(err)
	}

	// First put to db using the IPNS namespace.
	serializedRecord, err := proto.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}

	if err := n.Repo.Datastore().Put(namesys.IpnsDsKey(peerID), serializedRecord); err != nil {
		t.Fatal(err)
	}

	retreivedPath, err := getFromDatastore(n.Repo.Datastore(), peerID)
	if err != nil {
		t.Error(err)
	}

	if retreivedPath.String() != pth {
		t.Errorf("Retreived incorrect value. Expected %s, got %s", pth, retreivedPath.String())
	}

	// Next put to the database using the persistent cache namespace.
	if err := putToDatastoreCache(n.Repo.Datastore(), peerID, ipath.Path(pth)); err != nil {
		t.Fatal(err)
	}

	retreivedPath, err = getFromDatastore(n.Repo.Datastore(), peerID)
	if err != nil {
		t.Error(err)
	}

	if retreivedPath.String() != pth {
		t.Errorf("Retreived incorrect value. Expected %s, got %s", pth, retreivedPath.String())
	}

	// Test the persistent cache put did not override the IPNS namespace.
	ival, err := n.Repo.Datastore().Get(namesys.IpnsDsKey(peerID))
	if err != nil {
		t.Fatal(err)
	}

	rec2 := new(ipnspb.IpnsEntry)
	err = proto.Unmarshal(ival, rec2)
	if err != nil {
		t.Error(err)
	}
	if string(rec2.Value) != pth {
		t.Errorf("Retreived incorrect value. Expected %s, got %s", pth, string(rec2.Value))
	}
}
