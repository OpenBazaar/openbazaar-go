package discovery

import (
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dshelp "github.com/ipfs/go-ipfs-ds-help"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
)

type Local struct {
	ds datastore.Datastore
}

func NewLocal(ds datastore.Batching) *Local {
	return &Local{ds: ds}
}

func (l *Local) AddPeer(cid cid.Cid, peer retrievalmarket.RetrievalPeer) error {
	key := dshelp.MultihashToDsKey(cid.Hash())
	exists, err := l.ds.Has(key)
	if err != nil {
		return err
	}

	var newRecord []byte

	if !exists {
		newRecord, err = cbor.DumpObject([]retrievalmarket.RetrievalPeer{peer})
		if err != nil {
			return err
		}
	} else {
		entry, err := l.ds.Get(key)
		if err != nil {
			return err
		}
		var peerList []retrievalmarket.RetrievalPeer
		if err = cbor.DecodeInto(entry, &peerList); err != nil {
			return err
		}
		if hasPeer(peerList, peer) {
			return nil
		}
		peerList = append(peerList, peer)
		newRecord, err = cbor.DumpObject(peerList)
		if err != nil {
			return err
		}
	}

	return l.ds.Put(key, newRecord)
}

func hasPeer(peerList []retrievalmarket.RetrievalPeer, peer retrievalmarket.RetrievalPeer) bool {
	for _, p := range peerList {
		if p == peer {
			return true
		}
	}
	return false
}

func (l *Local) GetPeers(payloadCID cid.Cid) ([]retrievalmarket.RetrievalPeer, error) {
	entry, err := l.ds.Get(dshelp.MultihashToDsKey(payloadCID.Hash()))
	if err == datastore.ErrNotFound {
		return []retrievalmarket.RetrievalPeer{}, nil
	}
	if err != nil {
		return nil, err
	}
	var peerList []retrievalmarket.RetrievalPeer
	if err := cbor.DecodeInto(entry, &peerList); err != nil {
		return nil, err
	}
	return peerList, nil
}

var _ retrievalmarket.PeerResolver = &Local{}
