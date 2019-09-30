package db_test

import (
	"crypto/rand"

	ma "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	ps "gx/ipfs/QmaCTz9RkrU13bm9kMB54f7atgqM4qkjDZpRwRoJiWXEqs/go-libp2p-peerstore"
	multihash "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"

	"sync"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func mustNewPointer() ipfs.Pointer {
	randBytes := make([]byte, 32)
	_, err := rand.Read(randBytes)
	if err != nil {
		panic(err)
	}
	h, err := multihash.Encode(randBytes, multihash.SHA2_256)
	if err != nil {
		panic(err)
	}
	id, err := peer.IDFromBytes(h)
	if err != nil {
		panic(err)
	}
	maAddr, err := ma.NewMultiaddr("/ipfs/QmamudHQGtztShX7Nc9HcczehdpGGWpFBWu2JvKWcpELxr/")
	if err != nil {
		panic(err)
	}
	k, err := cid.Decode("QmamudHQGtztShX7Nc9HcczehdpGGWpFBWu2JvKWcpELxr")
	if err != nil {
		panic(err)
	}
	cancelID, err := peer.IDB58Decode("QmbwSMS35CaYKdrYBvvR9aHU9FzeWhjJ7E3jLKeR2DWrs3")
	if err != nil {
		panic(err)
	}
	return ipfs.Pointer{
		Cid: &k,
		Value: ps.PeerInfo{
			ID:    id,
			Addrs: []ma.Multiaddr{maAddr},
		},
		Purpose:   ipfs.MESSAGE,
		Timestamp: time.Now(),
		CancelID:  &cancelID,
	}
}

func buildNewPointerStore() (repo.PointerStore, func(), error) {
	appSchema := schema.MustNewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err := appSchema.BuildSchemaDirectories(); err != nil {
		return nil, nil, err
	}
	if err := appSchema.InitializeDatabase(); err != nil {
		return nil, nil, err
	}
	database, err := appSchema.OpenDatabase()
	if err != nil {
		return nil, nil, err
	}
	return db.NewPointerStore(database, new(sync.Mutex)), appSchema.DestroySchemaDirectories, nil
}

func TestPointersPut(t *testing.T) {
	pdb, teardown, err := buildNewPointerStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	pointer := mustNewPointer()
	err = pdb.Put(pointer)
	if err != nil {
		t.Error(err)
	}

	stmt, _ := pdb.PrepareQuery("select pointerID, key, address, cancelID, purpose, timestamp from pointers where pointerID=?")
	defer stmt.Close()

	var pointerID string
	var key string
	var address string
	var purpose int
	var timestamp int
	var cancelID string
	err = stmt.QueryRow(pointer.Value.ID.Pretty()).Scan(&pointerID, &key, &address, &cancelID, &purpose, &timestamp)
	if err != nil {
		t.Error(err)
	}
	if pointerID != pointer.Value.ID.Pretty() || timestamp <= 0 || key != pointer.Cid.String() || purpose != 1 || cancelID != pointer.CancelID.Pretty() {
		t.Error("Pointer returned incorrect values")
	}
	err = pdb.Put(pointer)
	if err == nil {
		t.Error("Allowed duplicate pointer")
	}
}

func TestDeletePointer(t *testing.T) {
	pdb, teardown, err := buildNewPointerStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	pointer := mustNewPointer()
	err = pdb.Put(pointer)
	if err != nil {
		t.Log(err)
	}
	err = pdb.Delete(pointer.Value.ID)
	if err != nil {
		t.Error("Pointer delete failed")
	}
	stmt, _ := pdb.PrepareQuery("select pointerID from pointers where pointerID=?")
	defer stmt.Close()

	var pointerID string
	err = stmt.QueryRow(pointer.Value.ID.Pretty()).Scan(&pointerID)
	if err == nil {
		t.Error("Pointer delete failed")
	}
}

func TestDeleteAllPointers(t *testing.T) {
	pdb, teardown, err := buildNewPointerStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	p := mustNewPointer()
	p.Purpose = ipfs.MODERATOR
	err = pdb.Put(p)
	if err != nil {
		t.Log(err)
	}
	err = pdb.DeleteAll(ipfs.MODERATOR)
	if err != nil {
		t.Error("Pointer delete failed")
	}
	stmt, _ := pdb.PrepareQuery("select pointerID from pointers where purpose=?")
	defer stmt.Close()

	var pointerID string
	err = stmt.QueryRow(ipfs.MODERATOR).Scan(&pointerID)
	if err == nil {
		t.Error("Pointer delete all failed")
	}
}

func TestGetAllPointers(t *testing.T) {
	pdb, teardown, err := buildNewPointerStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	pointer := mustNewPointer()
	err = pdb.Put(pointer)
	if err != nil {
		t.Log(err)
	}
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
		if !p.Cid.Equals(*pointer.Cid) {
			t.Error("Get all pointers returned incorrect data")
		}
		if p.CancelID.Pretty() != pointer.CancelID.Pretty() {
			t.Error("Get all pointers returned incorrect data")
		}
	}
}

func TestPointersDB_GetByPurpose(t *testing.T) {
	pdb, teardown, err := buildNewPointerStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = pdb.Put(mustNewPointer())
	if err != nil {
		t.Log(err)
	}
	randBytes := make([]byte, 32)
	_, err = rand.Read(randBytes)
	if err != nil {
		t.Log(err)
	}
	h, _ := multihash.Encode(randBytes, multihash.SHA2_256)
	id, _ := peer.IDFromBytes(h)
	maAddr, _ := ma.NewMultiaddr("/ipfs/QmamudHQGtztShX7Nc9HcczehdpGGWpFBWu2JvKWcpELxr/")
	k, _ := cid.Decode("QmamudHQGtztShX7Nc9HcczehdpGGWpFBWu2JvKWcpELxr")
	m := ipfs.Pointer{
		Cid: &k,
		Value: ps.PeerInfo{
			ID:    id,
			Addrs: []ma.Multiaddr{maAddr},
		},
		Purpose:   ipfs.MODERATOR,
		Timestamp: time.Now(),
		CancelID:  nil,
	}
	err = pdb.Put(m)
	if err != nil {
		t.Error("Put pointer returned error")
	}
	pointers, err := pdb.GetByPurpose(ipfs.MODERATOR)
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
		if !p.Cid.Equals(*m.Cid) {
			t.Error("Get pointers returned incorrect data")
		}
		if p.CancelID != nil {
			t.Error("Get pointers returned incorrect data")
		}
	}
}

func TestPointersDB_Get(t *testing.T) {
	pdb, teardown, err := buildNewPointerStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = pdb.Put(mustNewPointer())
	if err != nil {
		t.Log(err)
	}
	randBytes := make([]byte, 32)
	_, err = rand.Read(randBytes)
	if err != nil {
		t.Log(err)
	}
	h, _ := multihash.Encode(randBytes, multihash.SHA2_256)
	id, _ := peer.IDFromBytes(h)
	maAddr, _ := ma.NewMultiaddr("/ipfs/QmamudHQGtztShX7Nc9HcczehdpGGWpFBWu2JvKWcpELxr/")
	k, _ := cid.Decode("QmamudHQGtztShX7Nc9HcczehdpGGWpFBWu2JvKWcpELxr")
	m := ipfs.Pointer{
		Cid: &k,
		Value: ps.PeerInfo{
			ID:    id,
			Addrs: []ma.Multiaddr{maAddr},
		},
		Purpose:   ipfs.MODERATOR,
		Timestamp: time.Now(),
		CancelID:  nil,
	}
	err = pdb.Put(m)
	if err != nil {
		t.Error("Put pointer returned error")
	}
	p, err := pdb.Get(id)
	if err != nil {
		t.Error("Get pointers returned error")
	}

	if p.Purpose != m.Purpose {
		t.Error("Get pointers returned incorrect data")
	}
	if p.Value.ID != m.Value.ID {
		t.Error("Get pointers returned incorrect data")
	}
	if !p.Cid.Equals(*m.Cid) {
		t.Error("Get pointers returned incorrect data")
	}
	if p.CancelID != nil {
		t.Error("Get pointers returned incorrect data")
	}
}
