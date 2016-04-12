package api

import (
	"net/http"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	"github.com/gorilla/websocket"

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
		c.h.broadcast <- message
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
	ReadBufferSize: 1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request)bool{return true},
}

var handler wsHandler;

type wsHandler struct {
	h *hub
	node   *core.IpfsNode
	path string
	context commands.Context
}


func newWSAPIHandler(node *core.IpfsNode, ctx commands.Context) *wsHandler {
	h := newHub()
	go h.run()
	handler = wsHandler {
		h: h,
		node:   node,
		path: ctx.ConfigRoot,
		context: ctx,
	}
	return &handler
}

func (wsh wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("Error upgrading to websockets: ", err)
		return
	}
	c := &connection{send: make(chan []byte, 256), ws: ws, h: wsh.h}
	c.h.register <- c
	defer func() { c.h.unregister <- c }()
	go c.writer()
	c.reader()
}
