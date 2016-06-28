package libbitcoin

import (
	zmq "github.com/pebbe/zmq4"
	"github.com/anacrolix/sync"
)

type ZMQSocket struct {
	socket     *zmq.Socket
	socketType zmq.Type
	callback   chan Response
	publicKey  string
	secretKey  string
	lock       *sync.Mutex
}

type Response struct {
	data []byte
	more bool
}

func NewSocket(cb chan Response, socketType zmq.Type) *ZMQSocket {
	pub, secret, _ := zmq.NewCurveKeypair()
	l := new(sync.Mutex)
	socket := ZMQSocket{
		socketType: socketType,
		publicKey: pub,
		secretKey: secret,
		callback: cb,
		lock: l,
	}
	return &socket
}

func (s *ZMQSocket) Connect(address, publicKey string) error {
	sock, err := zmq.NewSocket(s.socketType)
	if err != nil {
		return err
	}
	if publicKey != "" {
		sock.SetCurvePublickey(s.publicKey)
		sock.SetCurveSecretkey(s.secretKey)
		sock.SetCurveServerkey(publicKey)
	}
	if s.socketType == zmq.SUB {
		sock.SetSubscribe("")
	}
	err = sock.Connect(address)
	s.socket = sock

	if err != nil {
		return err
	}
 	go s.poll()
	return nil
}

func (s *ZMQSocket) poll() {
	for {
		b, err := s.socket.RecvBytes(0)
		if err != nil {
			break
		}
		more, err := s.socket.GetRcvmore()
		if err != nil {
			break
		}
		if len(b) > 0 {
			r := Response{
				data: b,
				more: more,
			}
			s.callback <- r
		}
	}
}

func (s *ZMQSocket) Send(data []byte, flag zmq.Flag) {
	s.lock.Lock()
	s.socket.SendBytes(data, flag)
	s.lock.Unlock()
}

func (s *ZMQSocket) Close() {
	s.socket.Close()
}

func (s *ZMQSocket) ChangeEndpoint(current, newUrl, newPublicKey string){
	s.lock.Lock()
	if current != newUrl {
		s.socket.Disconnect(current)
		s.socket.Connect(newUrl)
		if newPublicKey != "" {
			s.socket.SetCurveServerkey(newPublicKey)
		}
	}
	s.lock.Unlock()
}
