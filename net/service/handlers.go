package service

import (
	peer "gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"
	"github.com/OpenBazaar/openbazaar-go/pb"
)

type serviceHandler func(peer.ID, *pb.Message) (*pb.Message, error)

func (service *OpenBazaarService) HandlerForMsgType(t pb.Message_MessageType) serviceHandler {
	switch t {
	case pb.Message_PING:
		return service.handlePing
	case pb.Message_FOLLOW:
		return service.handleFollow
	case pb.Message_UNFOLLOW:
		return service.handleUnFollow
	default:
		return nil
	}
}

func (service *OpenBazaarService) handlePing(peer peer.ID, pmes *pb.Message) (*pb.Message, error) {
	log.Debugf("Received PING message from %s", peer.Pretty())
	return pmes, nil
}

func (service *OpenBazaarService) handleFollow(peer peer.ID, pmes *pb.Message) (*pb.Message, error) {
	log.Debugf("Received FOLLOW message from %s", peer.Pretty())
	err := service.datastore.Followers().Put(peer.Pretty())
	if err != nil {
		return nil, err
	}
	service.broadcast <- []byte(`{"notification": {"follow":"` + peer.Pretty() + `"}}`)
	return nil, nil
}

func (service *OpenBazaarService) handleUnFollow(peer peer.ID, pmes *pb.Message) (*pb.Message, error) {
	log.Debugf("Received UNFOLLOW message from %s", peer.Pretty())
	err := service.datastore.Followers().Delete(peer.Pretty())
	if err != nil {
		return nil, err
	}
	service.broadcast <- []byte(`{"notification": {"unfollow":"` + peer.Pretty() + `"}}`)
	return nil, nil
}

func (service *OpenBazaarService) handleOfflineAck(p peer.ID, pmes *pb.Message) (*pb.Message, error) {
	log.Debugf("Received OFFLINE_ACK message from %s", p.Pretty())
	pid, err := peer.IDB58Decode(string(pmes.Payload.Value))
	if err != nil {
		return nil, err
	}
	err = service.datastore.Pointers().Delete(pid)
	if err != nil {
		return nil, err
	}
	return nil, nil
}