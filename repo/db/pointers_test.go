package db

import (
	"crypto/rand"
	"database/sql"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	ma "gx/ipfs/QmUAQaWbKxGCUTuoQVvvicbQNZ9APF5pDGWyAZSe93AtKH/go-multiaddr"
	multihash "gx/ipfs/QmYDds3421prZgqKbLpEK7T9Aa2eVdQ7o3YarX1LVLdP2J/go-multihash"
	cid "gx/ipfs/QmcTcsTvfaeEBRFo1TkFgT8sRmgi1n1LTZpecfVP8fzpGD/go-cid"
	ps "gx/ipfs/QmeXj9VAjmYQZxpmVz7VzccbJrpmr8qkCDSjfVNsPTWTYU/go-libp2p-peerstore"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
	"testing"
	"time"
)

var pdb PointersDB
var pointer ipfs.Pointer

func init() {
	conn, _ := sql.Open("sqlite3", ":memory:")
	initDatabaseTables(conn, "")
	pdb = PointersDB{
		db: conn,
	}
	randBytes := make([]byte, 32)
	rand.Read(randBytes)
	h, _ := multihash.Encode(randBytes, multihash.SHA2_256)
	id, _ := peer.IDFromBytes(h)
	maAddr, _ := ma.NewMultiaddr("/ipfs/QmamudHQGtztShX7Nc9HcczehdpGGWpFBWu2JvKWcpELxr/")
	k, _ := cid.Decode("QmamudHQGtztShX7Nc9HcczehdpGGWpFBWu2JvKWcpELxr")
	pointer = ipfs.Pointer{
		k,
		ps.PeerInfo{
			ID:    id,
			Addrs: []ma.Multiaddr{maAddr},
		},
		ipfs.MESSAGE,
		time.Now(),
	}
}

func TestPointersPut(t *testing.T) {

	err := pdb.Put(pointer)
	if err != nil {
		t.Error(err)
	}

	stmt, _ := pdb.db.Prepare("select pointerID, key, address, purpose, timestamp from pointers where pointerID=?")
	defer stmt.Close()

	var pointerID string
	var key string
	var address string
	var purpose int
	var timestamp int
	err = stmt.QueryRow(pointer.Value.ID.Pretty()).Scan(&pointerID, &key, &address, &purpose, &timestamp)
	if err != nil {
		t.Error(err)
	}
	if pointerID != pointer.Value.ID.Pretty() || timestamp <= 0 || key != pointer.Cid.String() || purpose != 1 {
		t.Error("Pointer returned incorrect values")
	}
	err = pdb.Put(pointer)
	if err == nil {
		t.Error("Allowed duplicate pointer")
	}
}

func TestDeletePointer(t *testing.T) {
	pdb.Put(pointer)
	err := pdb.Delete(pointer.Value.ID)
	if err != nil {
		t.Error("Pointer delete failed")
	}
	stmt, _ := pdb.db.Prepare("select pointerID, key, address, purpose, timestamp from pointers where pointerID=?")
	defer stmt.Close()

	var pointerID string
	var key string
	var address string
	var purpose int
	var timestamp int
	err = stmt.QueryRow(pointer.Value.ID.Pretty()).Scan(&pointerID, &key, &address, &purpose, &timestamp)
	if err == nil {
		t.Error("Pointer delete failed")
	}
}

func TestDeleteAllPointers(t *testing.T) {
	p := pointer
	p.Purpose = ipfs.MODERATOR
	pdb.Put(p)
	err := pdb.DeleteAll(ipfs.MODERATOR)
	if err != nil {
		t.Error("Pointer delete failed")
	}
	stmt, _ := pdb.db.Prepare("select pointerID, key, address, purpose, timestamp from pointers where purpose=?")
	defer stmt.Close()

	var pointerID string
	var key string
	var address string
	var purpose int
	var timestamp int
	err = stmt.QueryRow(ipfs.MODERATOR).Scan(&pointerID, &key, &address, &purpose, &timestamp)
	if err == nil {
		t.Error("Pointer delete all failed")
	}
}

func TestGetAllPointers(t *testing.T) {
	pdb.Put(pointer)
	pointers, err := pdb.GetAll()
	if err != nil {
		t.Error("Get all pointers returned error")
	}
	for _, p := range pointers {
		if p.Purpose != pointer.Purpose {
			t.Error("Get all pointers returned incorrect data")
		}
		if p.Value.ID != pointer.Value.ID {
			t.Error("Get all pointers returned incorrect data")
		}
		if !p.Cid.Equals(pointer.Cid) {
			t.Error("Get all pointers returned incorrect data")
		}
	}
}

func TestGetPointers(t *testing.T) {
	pdb.Put(pointer)
	randBytes := make([]byte, 32)
	rand.Read(randBytes)
	h, _ := multihash.Encode(randBytes, multihash.SHA2_256)
	id, _ := peer.IDFromBytes(h)
	maAddr, _ := ma.NewMultiaddr("/ipfs/QmamudHQGtztShX7Nc9HcczehdpGGWpFBWu2JvKWcpELxr/")
	k, _ := cid.Decode("QmamudHQGtztShX7Nc9HcczehdpGGWpFBWu2JvKWcpELxr")
	m := ipfs.Pointer{
		k,
		ps.PeerInfo{
			ID:    id,
			Addrs: []ma.Multiaddr{maAddr},
		},
		ipfs.MODERATOR,
		time.Now(),
	}
	err := pdb.Put(m)
	pointers, err := pdb.Get(ipfs.MODERATOR)
	if err != nil {
		t.Error("Get pointers returned error")
	}
	if len(pointers) != 1 {
		t.Error("Returned incorrect number of pointers")
	}
	for _, p := range pointers {
		if p.Purpose != m.Purpose {
			t.Error("Get pointers returned incorrect data")
		}
		if p.Value.ID != m.Value.ID {
			t.Error("Get pointers returned incorrect data")
		}
		if !p.Cid.Equals(m.Cid) {
			t.Error("Get pointers returned incorrect data")
		}
	}
}
