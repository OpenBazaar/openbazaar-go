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
	webrelays   []string
	peerID      string
	connections []*websocket.Conn
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

	// Establish connections
	var conns []*websocket.Conn
	for _, relay := range webrelays {

		// Connect and subscribe to websocket server
		conn, err := connectToServer(relay, sender)
		if err != nil {
			log.Error("Could not connect to: %s", relay)
		}

		conns = append(conns, conn)
	}
	return &WebRelayManager{webrelays, sender, conns}
}

func connectToServer(relay string, sender string) (*websocket.Conn, error) {
	// Generate subscription key for web relay
	peerIDMultihash, _ := multihash.FromB58String(sender)
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
		UserID:          sender,
		SubscriptionKey: base58.Encode(subscriptionKey),
	}

	data, _ := json.Marshal(subscribeMessage)
	typedmessage := TypedMessage{
		Type: "SubscribeMessage",
		Data: data,
	}
	fmt.Println("Sending SubscribeMessage:", typedmessage)

	socketmessage, _ := json.Marshal(typedmessage)

	fmt.Println(string(socketmessage))
	// Connect to websocket server
	fmt.Printf("Connecting to relay server: %s\n", relay)

	c, _, err := websocket.DefaultDialer.Dial(relay, nil)
	if err != nil {
		log.Fatal("dial:", err)
		return nil, err
	}

	err = c.WriteMessage(websocket.TextMessage, socketmessage)
	if err != nil {
		fmt.Println("write:", err)
		return nil, err
	}

	fmt.Printf("Successfully connected and subscribed to: %s\n", relay)

	go func() {
		for {
			// read in a message
			_, p, err := c.ReadMessage()
			if err != nil {
				fmt.Println(err)
				break
			}
			// print out that message for clarity
			fmt.Printf("Received incoming message from relay: %s\n", string(p))

			//if err := c.WriteMessage(messageType, p); err != nil {
			//	fmt.Println(err)
			//	//return nil, err
			//}

		}
	}()

	return c, nil
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

	for _, conn := range wrm.connections {
		err := conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			fmt.Println("write:", err)
			return
		}
		fmt.Printf("Successfully sent message to relay: %s\n", conn.RemoteAddr())
	}

}
