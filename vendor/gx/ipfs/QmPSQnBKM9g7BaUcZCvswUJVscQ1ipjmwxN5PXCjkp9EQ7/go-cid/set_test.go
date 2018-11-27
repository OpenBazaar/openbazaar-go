package cid

import (
	"crypto/rand"
	"errors"
	"testing"

	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
)

func makeRandomCid(t *testing.T) Cid {
	p := make([]byte, 256)
	_, err := rand.Read(p)
	if err != nil {
		t.Fatal(err)
	}

	h, err := mh.Sum(p, mh.SHA3, 4)
	if err != nil {
		t.Fatal(err)
	}

	cid := NewCidV1(7, h)

	return cid
}

func TestSet(t *testing.T) {
	cid := makeRandomCid(t)
	cid2 := makeRandomCid(t)
	s := NewSet()

	s.Add(cid)

	if !s.Has(cid) {
		t.Error("should have the CID")
	}

	if s.Len() != 1 {
		t.Error("should report 1 element")
	}

	keys := s.Keys()

	if len(keys) != 1 || !keys[0].Equals(cid) {
		t.Error("key should correspond to Cid")
	}

	if s.Visit(cid) {
		t.Error("visit should return false")
	}

	foreach := []Cid{}
	foreachF := func(c Cid) error {
		foreach = append(foreach, c)
		return nil
	}

	if err := s.ForEach(foreachF); err != nil {
		t.Error(err)
	}

	if len(foreach) != 1 {
		t.Error("ForEach should have visited 1 element")
	}

	foreachErr := func(c Cid) error {
		return errors.New("test")
	}

	if err := s.ForEach(foreachErr); err == nil {
		t.Error("Should have returned an error")
	}

	if !s.Visit(cid2) {
		t.Error("should have visited a new Cid")
	}

	if s.Len() != 2 {
		t.Error("len should be 2 now")
	}

	s.Remove(cid2)

	if s.Len() != 1 {
		t.Error("len should be 1 now")
	}
}
