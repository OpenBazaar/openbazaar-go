package net

import (
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/ipfs/go-ipfs/core"
	"golang.org/x/net/context"
)

type PointerRepublisher struct {
	ipfsNode *core.IpfsNode
	db       repo.Datastore
}

func NewPointerRepublisher(node *core.IpfsNode, database repo.Datastore) *PointerRepublisher {
	return &PointerRepublisher{
		ipfsNode: node,
		db:       database,
	}
}

func (r *PointerRepublisher) Run() {
	tick := time.NewTicker(time.Hour * 24)
	defer tick.Stop()
	go r.Republish()
	for range tick.C {
		go r.Republish()
	}
}

func (r *PointerRepublisher) Republish() {
	pointers, err := r.db.Pointers().GetAll()
	if err != nil {
		return
	}
	ctx := context.Background()
	for _, p := range pointers {
		if p.Purpose != ipfs.MESSAGE {
			ipfs.RePublishPointer(r.ipfsNode, ctx, p)
		} else {
			if time.Now().Sub(p.Timestamp) > time.Hour*24*30 {
				r.db.Pointers().Delete(p.Value.ID)
			} else {
				ipfs.RePublishPointer(r.ipfsNode, ctx, p)
			}
		}
	}
}
