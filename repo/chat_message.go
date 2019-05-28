package repo

type ChatMessage struct {
	MessageId string   `json:"messageId"`
	PeerId    string   `json:"peerId"`
	Subject   string   `json:"subject"`
	Message   string   `json:"message"`
	Read      bool     `json:"read"`
	Outgoing  bool     `json:"outgoing"`
	Timestamp *APITime `json:"timestamp"`
}

type ChatConversation struct {
	PeerId    string   `json:"peerId"`
	Unread    int      `json:"unread"`
	Last      string   `json:"lastMessage"`
	Timestamp *APITime `json:"timestamp"`
	Outgoing  bool     `json:"outgoing"`
}

type GroupChatMessage struct {
	PeerIds []string `json:"peerIds"`
	Subject string   `json:"subject"`
	Message string   `json:"message"`
}
