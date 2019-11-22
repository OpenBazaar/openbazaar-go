package factory

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/golang/protobuf/ptypes/any"
)

func NewMessageWithOrderPayload() repo.Message {
	payload := []byte("test payload")
	return repo.Message{
		Msg: pb.Message{
			MessageType: pb.Message_ORDER,
			Payload:     &any.Any{Value: payload},
		},
	}
}
