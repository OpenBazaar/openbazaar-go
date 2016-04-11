package api

import (
	"io"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	"golang.org/x/net/websocket"
)

type wsAPIHandler struct {
	node   *core.IpfsNode
	path string
	context commands.Context
}

var handler wsAPIHandler;

func newWSAPIHandler(node *core.IpfsNode, ctx commands.Context) (websocket.Handler, error) {
	handler = wsAPIHandler{
		node:   node,
		path: ctx.ConfigRoot,
		context: ctx,
	}
	return websocket.Handler(ServeWS), nil
}

func ServeWS(ws *websocket.Conn) {
	io.Copy(ws, ws)
}