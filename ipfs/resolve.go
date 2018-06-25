package ipfs

import (
	"context"
	"github.com/ipfs/go-ipfs/core"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"time"
)

// Publish a signed IPNS record to our Peer ID
func Resolve(n *core.IpfsNode, p peer.ID, timeout time.Duration) (string, error) {
	cctx, _ := context.WithTimeout(context.Background(), timeout)
	pth, err := n.Namesys.Resolve(cctx, "/ipns/"+p.Pretty())
	if err != nil {
		return "", err
	}
	return pth.Segments()[1], nil
}

func ResolveAltRoot(n *core.IpfsNode, p peer.ID, altRoot string, timeout time.Duration) (string, error) {
	cctx, _ := context.WithTimeout(context.Background(), timeout)
	pth, err := n.Namesys.Resolve(cctx, "/ipns/"+p.Pretty()+":"+altRoot)
	if err != nil {
		return "", err
	}
	return pth.Segments()[1], nil
}
