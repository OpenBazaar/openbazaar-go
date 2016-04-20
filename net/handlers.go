package net

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	peer "gx/ipfs/QmZwZjMVGss5rqYsJVGy18gNbkTJffFyq2x1uJ4e4p3ZAt/go-libp2p-peer"
)

type serviceHandler func(peer.ID, *pb.Message) (*pb.Message, error)

func (service *OpenBazaarService) handlerForMsgType(t pb.Message_MessageType) serviceHandler {
	switch t {
		case pb.Message_PING:
			return service.handlePing
		default:
			return nil
	}
}

func (service *OpenBazaarService) handlePing(peer peer.ID, pmes *pb.Message) (*pb.Message, error) {
	log.Debugf("Received PING message from %s", peer.Pretty())
	return pmes, nil
}