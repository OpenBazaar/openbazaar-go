package net

import (
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/ipfs/go-ipfs/core"
	"github.com/op/go-logging"
	"golang.org/x/net/context"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
)

var log = logging.MustGetLogger("service")

const kRepointFrequency = time.Hour * 12
const kPointerExpiration = time.Hour * 24 * 30

type PointerRepublisher struct {
	ipfsNode    *core.IpfsNode
	db          repo.Datastore
	pushNodes   []peer.ID
	isModerator func() bool
}

func NewPointerRepublisher(node *core.IpfsNode, database repo.Datastore, pushNodes []peer.ID, isModerator func() bool) *PointerRepublisher {
	return &PointerRepublisher{
		ipfsNode:    node,
		db:          database,
		pushNodes:   pushNodes,
		isModerator: isModerator,
	}
}

func (r *PointerRepublisher) Run() {
	tick := time.NewTicker(kRepointFrequency)
	defer tick.Stop()
	go r.Republish()
	for range tick.C {
		go r.Republish()
	}
}

func (r *PointerRepublisher) Republish() {
	republishModerator := r.isModerator()
	pointers, err := r.db.Pointers().GetAll()
	if err != nil {
		log.Error(err)
		return
	}
	ctx := context.Background()

	for _, p := range pointers {
		switch p.Purpose {
		case ipfs.MESSAGE:
			if time.Now().Sub(p.Timestamp) > kPointerExpiration {
				r.db.Pointers().Delete(p.Value.ID)
			} else {
				go ipfs.PublishPointer(r.ipfsNode, ctx, p)
				for _, peer := range r.pushNodes {
					go ipfs.PutPointerToPeer(r.ipfsNode, context.Background(), peer, p)
				}
			}
		case ipfs.MODERATOR:
			if republishModerator {
				go ipfs.PublishPointer(r.ipfsNode, ctx, p)
			} else {
				r.db.Pointers().Delete(p.Value.ID)
			}
		default:
			continue
		}
	}
}
