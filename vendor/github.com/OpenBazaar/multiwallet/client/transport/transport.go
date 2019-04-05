package transport

import (
	"errors"
	tp "github.com/OpenBazaar/golang-socketio/transport"
	"github.com/gorilla/websocket"
	"golang.org/x/net/proxy"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

const (
	upgradeFailed = "Upgrade failed: "

	WsDefaultPingInterval   = 30 * time.Second
	WsDefaultPingTimeout    = 60 * time.Second
	WsDefaultReceiveTimeout = 60 * time.Second
	WsDefaultSendTimeout    = 60 * time.Second
	WsDefaultBufferSize     = 1024 * 32
)

var (
	ErrorBinaryMessage     = errors.New("Binary messages are not supported")
	ErrorBadBuffer         = errors.New("Buffer error")
	ErrorPacketWrong       = errors.New("Wrong packet type error")
	ErrorMethodNotAllowed  = errors.New("Method not allowed")
	ErrorHttpUpgradeFailed = errors.New("Http upgrade failed")
)

type WebsocketConnection struct {
	socket    *websocket.Conn
	transport *WebsocketTransport
}

func (wsc *WebsocketConnection) GetMessage() (message string, err error) {
	wsc.socket.SetReadDeadline(time.Now().Add(wsc.transport.ReceiveTimeout))
	msgType, reader, err := wsc.socket.NextReader()
	if err != nil {
		return "", err
	}

	//support only text messages exchange
	if msgType != websocket.TextMessage {
		return "", ErrorBinaryMessage
	}

	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", ErrorBadBuffer
	}
	text := string(data)

	//empty messages are not allowed
	if len(text) == 0 {
		return "", ErrorPacketWrong
	}

	return text, nil
}

func (wsc *WebsocketConnection) WriteMessage(message string) error {
	wsc.socket.SetWriteDeadline(time.Now().Add(wsc.transport.SendTimeout))
	writer, err := wsc.socket.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	if _, err := writer.Write([]byte(message)); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return nil
}

func (wsc *WebsocketConnection) Close() {
	wsc.socket.Close()
}

func (wsc *WebsocketConnection) PingParams() (interval, timeout time.Duration) {
	return wsc.transport.PingInterval, wsc.transport.PingTimeout
}

type WebsocketTransport struct {
	PingInterval   time.Duration
	PingTimeout    time.Duration
	ReceiveTimeout time.Duration
	SendTimeout    time.Duration

	BufferSize int

	RequestHeader http.Header

	proxyDialer proxy.Dialer
}

func (wst *WebsocketTransport) Connect(url string) (conn tp.Connection, err error) {
	dial := net.Dial
	if wst.proxyDialer != nil {
		dial = wst.proxyDialer.Dial
	}
	dialer := websocket.Dialer{NetDial: dial}
	socket, _, err := dialer.Dial(url, wst.RequestHeader)
	if err != nil {
		return nil, err
	}

	return &WebsocketConnection{socket, wst}, nil
}

func (wst *WebsocketTransport) HandleConnection(
	w http.ResponseWriter, r *http.Request) (conn tp.Connection, err error) {

	if r.Method != "GET" {
		http.Error(w, upgradeFailed+ErrorMethodNotAllowed.Error(), 503)
		return nil, ErrorMethodNotAllowed
	}

	socket, err := websocket.Upgrade(w, r, nil, wst.BufferSize, wst.BufferSize)
	if err != nil {
		http.Error(w, upgradeFailed+err.Error(), 503)
		return nil, ErrorHttpUpgradeFailed
	}

	return &WebsocketConnection{socket, wst}, nil
}

/**
Websocket connection do not require any additional processing
*/
func (wst *WebsocketTransport) Serve(w http.ResponseWriter, r *http.Request) {}

/**
Returns websocket connection with default params
*/
func GetDefaultWebsocketTransport(proxyDialer proxy.Dialer) *WebsocketTransport {
	return &WebsocketTransport{
		PingInterval:   WsDefaultPingInterval,
		PingTimeout:    WsDefaultPingTimeout,
		ReceiveTimeout: WsDefaultReceiveTimeout,
		SendTimeout:    WsDefaultSendTimeout,
		BufferSize:     WsDefaultBufferSize,
		proxyDialer:    proxyDialer,
	}
}
