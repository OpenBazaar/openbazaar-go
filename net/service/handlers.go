package service

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	peer "gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	"gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
)

func (service *OpenBazaarService) HandlerForMsgType(t pb.Message_MessageType) func(peer.ID, *pb.Message) (*pb.Message, error) {
	switch t {
	case pb.Message_PING:
		return service.handlePing
	case pb.Message_FOLLOW:
		return service.handleFollow
	case pb.Message_UNFOLLOW:
		return service.handleUnFollow
	case pb.Message_OFFLINE_ACK:
		return service.handleOfflineAck
	case pb.Message_ORDER:
		return service.handleOrder
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

func (service *OpenBazaarService) handleOrder(peer peer.ID, pmes *pb.Message) (*pb.Message, error) {
	log.Debugf("Received ORDER message from %s", peer.Pretty())
	errorResponse := func(error string) *pb.Message {
		a := &any.Any{Value: []byte(error)}
		m := &pb.Message{
			MessageType: pb.Message_ERROR,
			Payload:     a,
		}
		return m
	}
	contract := new(pb.RicardianContract)
	err := proto.Unmarshal(pmes.Payload.Value, contract)
	if err != nil {
		return errorResponse("Could not unmarshal order"), nil
	}
	err = service.node.ValidateOrder(contract)
	if err != nil {
		return errorResponse(err.Error()), nil
	}

	contract, err = service.node.NewOrderConfirmation(contract)
	if err != nil {
		return errorResponse("Error building order confirmation"), nil
	}
	a, err := ptypes.MarshalAny(contract)
	if err != nil {
		return errorResponse("Error building order confirmation"), nil
	}
	service.node.Datastore.Sales().Put(contract.VendorOrderConfirmation.OrderID, *contract, pb.OrderState_CONFIRMED, false)
	m := pb.Message{
		MessageType: pb.Message_ORDER_CONFIRMATION,
		Payload:     a,
	}
	return &m, nil
}
