package net

import (
	"time"
	"github.com/ipfs/go-ipfs/core"
	"golang.org/x/net/context"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
)

type PointerRepublisher struct {
	ipfsNode *core.IpfsNode
	db       repo.Datastore
}

func NewPointerRepublisher(node *core.IpfsNode, database repo.Datastore) *PointerRepublisher{
	return &PointerRepublisher{
		ipfsNode: node,
		db: database,
	}
}

func (r *PointerRepublisher) Run() {
	tick := time.NewTicker(time.Hour * 24)
	defer tick.Stop()
	go r.republish()
	for {
		select {
		case <-tick.C:
			go r.republish()
		}
	}
}

func (r *PointerRepublisher) republish() {
	pointers, err := r.db.Pointers().GetAll()
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, p := range pointers {
		if p.Purpose != ipfs.MESSAGE {
			ipfs.RePublishPointer(r.ipfsNode, ctx, p)
		} else {
			if time.Now().Sub(p.Timestamp) > time.Hour * 24 * 30 {
				r.db.Pointers().Delete(p.Value.ID)
			} else {
				ipfs.RePublishPointer(r.ipfsNode, ctx, p)
			}
		}
	}
}