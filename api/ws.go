package api

import (
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/gorilla/websocket"
	"github.com/ipfs/go-ipfs/commands"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	"net/http"
)

type connection struct {
	// The websocket connection.
	ws *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	// The hub.
	h *hub
}

func (c *connection) reader() {
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		log.Debugf("Incoming websocket message: %s", string(message))

		// Just echo for now until we set up the api
		c.h.Broadcast <- message
	}
	c.ws.Close()
}

func (c *connection) writer() {
	for message := range c.send {
		err := c.ws.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			break
		}
	}
	c.ws.Close()
}

var upgrader = &websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

var handler wsHandler

type wsHandler struct {
	h             *hub
	path          string
	context       commands.Context
	authenticated bool
	cookieJar     []http.Cookie
}

func newWSAPIHandler(node *core.OpenBazaarNode, ctx commands.Context, cookieJar []http.Cookie) (*wsHandler, error) {
	cfg, err := node.Context.GetConfig()
	if err != nil {
		return nil, err
	}

	gatewayMaddr, err := ma.NewMultiaddr(cfg.Addresses.Gateway)
	if err != nil {
		return nil, err
	}
	addr, err := gatewayMaddr.ValueForProtocol(ma.P_IP4)
	if err != nil {
		return nil, err
	}
	var authenticated bool
	if addr != "127.0.0.1" {
		authenticated = true
	}

	hub := newHub()
	go hub.run()
	handler = wsHandler{
		h:             hub,
		path:          ctx.ConfigRoot,
		context:       ctx,
		authenticated: authenticated,
		cookieJar:     cookieJar,
	}
	return &handler, nil
}

func (wsh wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("Error upgrading to websockets:", err)
		return
	}
	if wsh.authenticated {
		cookie, err := r.Cookie("OpenBazaarSession")
		if err != nil {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var auth bool
		for _, key := range wsh.cookieJar {
			if key.Value == cookie.Value {
				auth = true
				break
			}
		}
		if !auth {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}
	c := &connection{send: make(chan []byte, 256), ws: ws, h: wsh.h}
	c.h.register <- c
	defer func() { c.h.unregister <- c }()
	go c.writer()
	c.reader()
}
