package protocol

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

const (
	open          = "0"
	msg           = "4"
	emptyMessage  = "40"
	commonMessage = "42"
	ackMessage    = "43"

	CloseMessage = "1"
	PingMessage = "2"
	PongMessage = "3"
)

var (
	ErrorWrongMessageType = errors.New("Wrong message type")
	ErrorWrongPacket      = errors.New("Wrong packet")
)

func typeToText(msgType int) (string, error) {
	switch msgType {
	case MessageTypeOpen:
		return open, nil
	case MessageTypeClose:
		return CloseMessage, nil
	case MessageTypePing:
		return PingMessage, nil
	case MessageTypePong:
		return PongMessage, nil
	case MessageTypeEmpty:
		return emptyMessage, nil
	case MessageTypeEmit, MessageTypeAckRequest:
		return commonMessage, nil
	case MessageTypeAckResponse:
		return ackMessage, nil
	}
	return "", ErrorWrongMessageType
}

func Encode(msg *Message) (string, error) {
	result, err := typeToText(msg.Type)
	if err != nil {
		return "", err
	}

	if msg.Type == MessageTypeEmpty || msg.Type == MessageTypePing ||
		msg.Type == MessageTypePong {
		return result, nil
	}

	if msg.Type == MessageTypeAckRequest || msg.Type == MessageTypeAckResponse {
		result += strconv.Itoa(msg.AckId)
	}

	if msg.Type == MessageTypeOpen || msg.Type == MessageTypeClose {
		return result + msg.Args, nil
	}

	if msg.Type == MessageTypeAckResponse {
		return result + "[" + msg.Args + "]", nil
	}

	jsonMethod, err := json.Marshal(&msg.Method)
	if err != nil {
		return "", err
	}

	return result + "[" + string(jsonMethod) + "," + msg.Args + "]", nil
}

func MustEncode(msg *Message) string {
	result, err := Encode(msg)
	if err != nil {
		panic(err)
	}

	return result
}

func getMessageType(data string) (int, error) {
	if len(data) == 0 {
		return 0, ErrorWrongMessageType
	}
	switch data[0:1] {
	case open:
		return MessageTypeOpen, nil
	case CloseMessage:
		return MessageTypeClose, nil
	case PingMessage:
		return MessageTypePing, nil
	case PongMessage:
		return MessageTypePong, nil
	case msg:
		if len(data) == 1 {
			return 0, ErrorWrongMessageType
		}
		switch data[0:2] {
		case emptyMessage:
			return MessageTypeEmpty, nil
		case commonMessage:
			return MessageTypeAckRequest, nil
		case ackMessage:
			return MessageTypeAckResponse, nil
		}
	}
	return 0, ErrorWrongMessageType
}

/**
Get ack id of current packet, if present
*/
func getAck(text string) (ackId int, restText string, err error) {
	if len(text) < 4 {
		return 0, "", ErrorWrongPacket
	}
	text = text[2:]

	pos := strings.IndexByte(text, '[')
	if pos == -1 {
		return 0, "", ErrorWrongPacket
	}

	ack, err := strconv.Atoi(text[0:pos])
	if err != nil {
		return 0, "", err
	}

	return ack, text[pos:], nil
}

/**
Get message method of current packet, if present
*/
func getMethod(text string) (method, restText string, err error) {
	var start, end, rest, countQuote int

	for i, c := range text {
		if c == '"' {
			switch countQuote {
			case 0:
				start = i + 1
			case 1:
				end = i
				rest = i + 1
			default:
				return "", "", ErrorWrongPacket
			}
			countQuote++
		}
		if c == ',' {
			if countQuote < 2 {
				continue
			}
			rest = i + 1
			break
		}
	}

	if (end < start) || (rest >= len(text)) {
		return "", "", ErrorWrongPacket
	}

	return text[start:end], text[rest : len(text)-1], nil
}

func Decode(data string) (*Message, error) {
	var err error
	msg := &Message{}
	msg.Source = data

	msg.Type, err = getMessageType(data)
	if err != nil {
		return nil, err
	}

	if msg.Type == MessageTypeOpen {
		msg.Args = data[1:]
		return msg, nil
	}

	if msg.Type == MessageTypeClose || msg.Type == MessageTypePing ||
		msg.Type == MessageTypePong || msg.Type == MessageTypeEmpty {
		return msg, nil
	}

	ack, rest, err := getAck(data)
	msg.AckId = ack
	if msg.Type == MessageTypeAckResponse {
		if err != nil {
			return nil, err
		}
		msg.Args = rest[1 : len(rest)-1]
		return msg, nil
	}

	if err != nil {
		msg.Type = MessageTypeEmit
		rest = data[2:]
	}

	msg.Method, msg.Args, err = getMethod(rest)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func ToArgArray(arg interface{})(i []interface{}) {
	i = append(i, arg)
	return i
}
