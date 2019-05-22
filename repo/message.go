package repo

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/golang/protobuf/ptypes/any"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

var ErrUnknownMessage = errors.New("unknown or invalid message")

type Message struct {
	Msg pb.Message
}

func (m *Message) MarshalJSON() ([]byte, error) {
	fmt.Println("in marshal json")
	return json.Marshal(m.Msg)
}

func (m *Message) UnmarshalJSON(b []byte) error {
	fmt.Println("in unmarshal json")
	return json.Unmarshal(b, &m.Msg)
}

func (m *Message) GetMessageType() pb.Message_MessageType {
	return m.Msg.MessageType
}

func (m *Message) GetPayload() *any.Any {
	return m.Msg.Payload
}
