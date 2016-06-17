package libbitcoin

import (
	"time"
	"math/rand"
	"encoding/binary"
	zmq "github.com/pebbe/zmq4"
)
const MAX_UNIT32 = 4294967295

type ClientBase struct {
	socket         *ZMQSocket
	outstanding    map[int]outstanding
	messages       [][]byte
	handler        chan Response
	parser         func(command string, data []byte, callback func(interface{}, error))
	timeout        func()
}

type outstanding struct {
	stop     chan interface{}
	callback func(interface{}, error)
}

func NewClientBase(address string, publicKey string) *ClientBase {
	handler := make(chan Response)
	o := make(map[int]outstanding)
	cb := ClientBase{
		socket: NewSocket(handler, zmq.DEALER),
		handler: handler,
		outstanding: o,
		messages: [][]byte{},
	}
	cb.socket.Connect(address, publicKey)
	go cb.handleResponse()
	return &cb
}

func (cb *ClientBase) SendCommand(command string, data []byte, callback func(interface{}, error)) {
	txid := rand.Intn(MAX_UNIT32)
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(txid))

	cb.socket.Send([]byte(command), 2)
	cb.socket.Send(b, 2)
	cb.socket.Send(data, 0)

	ticker := time.NewTicker(10 * time.Second)
	c := make(chan interface{})
	cb.outstanding[txid] = outstanding{
		callback: callback,
		stop: c,
	}
	listen:
		for {
			select {
			case <- c: // Server returned properly.
				ticker.Stop()
				break listen
			case <- ticker.C: //Server timed out. Rotate servers and resend message.
				log.Warningf("Libbitcoin server timed out on %s\n", command)
				ticker.Stop()
				cb.timeout()
				_, ok := cb.outstanding[txid]
				if ok {
					delete(cb.outstanding, txid)
				}
				cb.SendCommand(command, data, callback)
				break listen
			}
		}
}

func (cb *ClientBase) messageReceived(command string, id, data []byte){
	txid := int(binary.LittleEndian.Uint32(id))
	var callback func(interface{}, error)
	if _, ok := cb.outstanding[txid]; ok {
		cb.outstanding[txid].stop <- ""
		callback = cb.outstanding[txid].callback
		delete(cb.outstanding, txid)
	}
	cb.parser(command, data, callback)
}

func (cb *ClientBase) handleResponse(){
	for r := range cb.handler {
		cb.messages = append(cb.messages, r.data)
		if !r.more {
			command := string(cb.messages[0])
			id := cb.messages[1]
			data := cb.messages[2]
			cb.messageReceived(command, id, data)
			cb.messages = [][]byte{}
		}
	}
}


