package test

import (
	"sort"
	"testing"

	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	pt "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer/test"

	pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
)

var keyBookSuite = map[string]func(kb pstore.KeyBook) func(*testing.T){
	"AddGetPrivKey":         testKeybookPrivKey,
	"AddGetPubKey":          testKeyBookPubKey,
	"PeersWithKeys":         testKeyBookPeers,
	"PubKeyAddedOnRetrieve": testInlinedPubKeyAddedOnRetrieve,
}

type KeyBookFactory func() (pstore.KeyBook, func())

func TestKeyBook(t *testing.T, factory KeyBookFactory) {
	for name, test := range keyBookSuite {
		// Create a new peerstore.
		kb, closeFunc := factory()

		// Run the test.
		t.Run(name, test(kb))

		// Cleanup.
		if closeFunc != nil {
			closeFunc()
		}
	}
}

func testKeybookPrivKey(kb pstore.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		if peers := kb.PeersWithKeys(); len(peers) > 0 {
			t.Error("expected peers to be empty on init")
		}

		priv, _, err := pt.RandTestKeyPair(512)
		if err != nil {
			t.Error(err)
		}

		id, err := peer.IDFromPrivateKey(priv)
		if err != nil {
			t.Error(err)
		}

		if res := kb.PrivKey(id); res != nil {
			t.Error("retrieving private key should have failed")
		}

		err = kb.AddPrivKey(id, priv)
		if err != nil {
			t.Error(err)
		}

		if res := kb.PrivKey(id); !priv.Equals(res) {
			t.Error("retrieved private key did not match stored private key")
		}

		if peers := kb.PeersWithKeys(); len(peers) != 1 || peers[0] != id {
			t.Error("list of peers did not include test peer")
		}
	}
}

func testKeyBookPubKey(kb pstore.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		if peers := kb.PeersWithKeys(); len(peers) > 0 {
			t.Error("expected peers to be empty on init")
		}

		_, pub, err := pt.RandTestKeyPair(512)
		if err != nil {
			t.Error(err)
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			t.Error(err)
		}

		if res := kb.PubKey(id); res != nil {
			t.Error("retrieving public key should have failed")
		}

		err = kb.AddPubKey(id, pub)
		if err != nil {
			t.Error(err)
		}

		if res := kb.PubKey(id); !pub.Equals(res) {
			t.Error("retrieved public key did not match stored public key")
		}

		if peers := kb.PeersWithKeys(); len(peers) != 1 || peers[0] != id {
			t.Error("list of peers did not include test peer")
		}
	}
}

func testKeyBookPeers(kb pstore.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		if peers := kb.PeersWithKeys(); len(peers) > 0 {
			t.Error("expected peers to be empty on init")
		}

		var peers peer.IDSlice
		for i := 0; i < 10; i++ {
			// Add a public key.
			_, pub, _ := pt.RandTestKeyPair(512)
			p1, _ := peer.IDFromPublicKey(pub)
			kb.AddPubKey(p1, pub)

			// Add a private key.
			priv, _, _ := pt.RandTestKeyPair(512)
			p2, _ := peer.IDFromPrivateKey(priv)
			kb.AddPrivKey(p2, priv)

			peers = append(peers, []peer.ID{p1, p2}...)
		}

		kbPeers := kb.PeersWithKeys()
		sort.Sort(kbPeers)
		sort.Sort(peers)

		for i, p := range kbPeers {
			if p != peers[i] {
				t.Errorf("mismatch of peer at index %d", i)
			}
		}
	}
}

func testInlinedPubKeyAddedOnRetrieve(kb pstore.KeyBook) func(t *testing.T) {
	return func(t *testing.T) {
		if peers := kb.PeersWithKeys(); len(peers) > 0 {
			t.Error("expected peers to be empty on init")
		}

		// Key small enough for inlining.
		_, pub, err := pt.RandTestKeyPair(32)
		if err != nil {
			t.Error(err)
		}

		id, err := peer.IDFromPublicKey(pub)
		if err != nil {
			t.Error(err)
		}

		pubKey := kb.PubKey(id)
		if !pubKey.Equals(pub) {
			t.Error("mismatch between original public key and keybook-calculated one")
		}
	}
}
