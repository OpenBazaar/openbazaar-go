package repo

import "time"

type ChatMessage struct {
	MessageId int       `json:"messageId"`
	PeerId    string    `json:"peerId"`
	Subject   string    `json:"subject"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	Outgoing  bool      `json:"outgoing"`
	Timestamp time.Time `json:"timestamp"`
}

type ChatConversation struct {
	PeerId string `json:"peerId"`
	Unread int    `json:"unread"`
}
