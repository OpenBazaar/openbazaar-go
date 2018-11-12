package ifconnmgr

import (
	"context"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	inet "gx/ipfs/QmXuRkCR7BNQa9uqfpTiFWsTQLzmTWYg91Ja1w95gnqb6u/go-libp2p-net"
)

type NullConnMgr struct{}

func (_ NullConnMgr) TagPeer(peer.ID, string, int)  {}
func (_ NullConnMgr) UntagPeer(peer.ID, string)     {}
func (_ NullConnMgr) GetTagInfo(peer.ID) *TagInfo   { return &TagInfo{} }
func (_ NullConnMgr) TrimOpenConns(context.Context) {}
func (_ NullConnMgr) Notifee() inet.Notifiee        { return &cmNotifee{} }

type cmNotifee struct{}

func (nn *cmNotifee) Connected(n inet.Network, c inet.Conn)         {}
func (nn *cmNotifee) Disconnected(n inet.Network, c inet.Conn)      {}
func (nn *cmNotifee) Listen(n inet.Network, addr ma.Multiaddr)      {}
func (nn *cmNotifee) ListenClose(n inet.Network, addr ma.Multiaddr) {}
func (nn *cmNotifee) OpenedStream(inet.Network, inet.Stream)        {}
func (nn *cmNotifee) ClosedStream(inet.Network, inet.Stream)        {}
