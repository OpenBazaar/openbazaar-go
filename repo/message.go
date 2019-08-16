package repo

import (
	"encoding/json"
	"errors"

	"github.com/golang/protobuf/ptypes/any"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

// ErrUnknownMessage - notify an invalid message
var ErrUnknownMessage = errors.New("unknown or invalid message")

// Message - wrapper for pb.Message
type Message struct {
	Msg pb.Message
}

// MarshalJSON - invoke the pb.Message marshaller
func (m *Message) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Msg)
}

// UnmarshalJSON - invoke the pb.Message unmarshaller
func (m *Message) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &m.Msg)
}

// GetMessageType - return the pb.Message messageType
func (m *Message) GetMessageType() pb.Message_MessageType {
	return m.Msg.MessageType
}

// GetPayload - return the pb.Message payload
func (m *Message) GetPayload() *any.Any {
	return m.Msg.Payload
}
