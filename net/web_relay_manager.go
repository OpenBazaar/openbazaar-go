package net

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/btcsuite/btcutil/base58"

	"gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"

	"github.com/gorilla/websocket"
)

type WebRelayManager struct {
	webrelays []string
	peerID    string
}

type EncryptedMessage struct {
	Message   string `json:"encryptedMessage"`
	Recipient string `json:"recipient"`
}

type TypedMessage struct {
	Type string
	Data json.RawMessage
}

type SubscribeMessage struct {
	UserID          string `json:"userID"`
	SubscriptionKey string `json:"subscriptionKey"`
}

func NewWebRelayManager(webrelays []string, sender string) *WebRelayManager {
	return &WebRelayManager{webrelays, sender}
}

func (wrm *WebRelayManager) SendRelayMessage(ciphertext string, recipient string) {
	encryptedmessage := EncryptedMessage{
		Message:   ciphertext,
		Recipient: recipient,
	}

	data, _ := json.Marshal(encryptedmessage)

	typedmessage := TypedMessage{
		Type: "EncryptedMessage",
		Data: data,
	}

	outgoing, _ := json.Marshal(typedmessage)
	fmt.Println(string(outgoing))

	// Transmit the encrypted message to the webrelay
	wrm.authToWebRelay(wrm.webrelays[0], outgoing)

}

func (wrm *WebRelayManager) authToWebRelay(server string, msg []byte) {

	// Generate subscription key for web relay
	peerIDMultihash, _ := multihash.FromB58String(wrm.peerID)
	decoded, _ := multihash.Decode(peerIDMultihash)
	digest := decoded.Digest
	prefix := digest[:8]

	prefix64 := binary.BigEndian.Uint64(prefix)

	// Then shifting
	shiftedPrefix64 := prefix64 >> uint(48)

	// Then converting back to a byte array
	shiftedBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(shiftedBytes, shiftedPrefix64)

	hashedShiftedPrefix := sha256.Sum256(shiftedBytes)

	subscriptionKey, _ := multihash.Encode(hashedShiftedPrefix[:], multihash.SHA2_256)

	// Generate subscribe message
	subscribeMessage := SubscribeMessage{
		UserID:          wrm.peerID,
		SubscriptionKey: base58.Encode(subscriptionKey),
	}

	data, _ := json.Marshal(subscribeMessage)
	typedmessage := TypedMessage{
		Type: "SubscribeMessage",
		Data: data,
	}
	fmt.Println(typedmessage)

	socketmessage, _ := json.Marshal(typedmessage)

	// Connect to websocket server
	fmt.Printf("connecting to %s\n", server)

	c, _, err := websocket.DefaultDialer.Dial(server, nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	err = c.WriteMessage(websocket.TextMessage, socketmessage)
	if err != nil {
		fmt.Println("write:", err)
		return
	}

	err = c.WriteMessage(websocket.TextMessage, msg)
	if err != nil {
		fmt.Println("write:", err)
		return
	}

}
