package api

type Hub struct {
	// Registered connections.
	connections map[*connection]bool

	// Inbound messages from the connections.
	Broadcast chan []byte

	// Register requests from the connections.
	register chan *connection

	// Unregister requests from connections.
	unregister chan *connection
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:   make(chan []byte),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		connections: make(map[*connection]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
			case c := <-h.register:
				h.connections[c] = true
				log.Debug("Registered new websocket connection")
			case c := <-h.unregister:
				if _, ok := h.connections[c]; ok {
					delete(h.connections, c)
					close(c.send)
				}
				log.Debug("Unregistered websocket connection")
			case m := <-h.Broadcast:
				for c := range h.connections {
					select {
						case c.send <- m:
						default:
							delete(h.connections, c)
							close(c.send)
					}
				}
		}
	}
}
