package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/gorilla/websocket"
	"github.com/op/go-logging"
)

type connection struct {
	// The websocket connection
	ws *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte

	// The hub
	h *hub
}

func (c *connection) reader() {
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		log.Debugf("Incoming websocket message: %s", string(message))

		// Just echo for now until we set up the API
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
	enabled       bool
	authenticated bool
	allowedIPs    map[string]bool
	cookie        http.Cookie
	username      string
	password      string
	logger        *logging.Logger
}

func newWSAPIHandler(node *core.OpenBazaarNode, authCookie http.Cookie, config schema.APIConfig) *wsHandler {
	hub := newHub()
	go hub.run()
	allowedIps := make(map[string]bool)
	for _, ip := range config.AllowedIPs {
		allowedIps[ip] = true
	}
	handler = wsHandler{
		h:             hub,
		path:          node.RepoPath,
		enabled:       config.Enabled,
		authenticated: config.Authenticated,
		allowedIPs:    allowedIps,
		cookie:        authCookie,
		username:      config.Username,
		password:      config.Password,
		logger:        logging.MustGetLogger("api"),
	}
	return &handler
}

func (wsh wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !wsh.enabled {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "403 - Forbidden")
		return
	}
	if len(wsh.allowedIPs) > 0 {
		remoteAddr := strings.Split(r.RemoteAddr, ":")
		if !wsh.allowedIPs[remoteAddr[0]] {
			wsh.logger.Errorf("refused websocket connection from ip: %s", remoteAddr[0])
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "403 - Forbidden")
			return
		}
	}
	if wsh.authenticated {
		if wsh.username == "" || wsh.password == "" {
			cookie, err := r.Cookie("OpenBazaar_Auth_Cookie")
			if err != nil {
				wsh.logger.Error("refused websocket connection: no cookie present")
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, "403 - Forbidden")
				return
			}
			if wsh.cookie.Value != cookie.Value {
				wsh.logger.Error("refused websocket connection: invalid cookie")
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, "403 - Forbidden")
				return
			}
		} else {
			username, password, ok := r.BasicAuth()
			h := sha256.Sum256([]byte(password))
			password = hex.EncodeToString(h[:])
			if !ok || username != wsh.username || strings.ToLower(password) != strings.ToLower(wsh.password) {
				wsh.logger.Error("refused websocket connection: invalid username and/or password")
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, "403 - Forbidden")
				return
			}
		}
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		wsh.logger.Error("upgrading websocket:", err)
		return
	}
	wsh.logger.Info("websocket connection established")
	c := &connection{send: make(chan []byte, 256), ws: ws, h: wsh.h}
	c.h.register <- c
	defer func() { c.h.unregister <- c }()
	go c.writer()
	c.reader()
}
