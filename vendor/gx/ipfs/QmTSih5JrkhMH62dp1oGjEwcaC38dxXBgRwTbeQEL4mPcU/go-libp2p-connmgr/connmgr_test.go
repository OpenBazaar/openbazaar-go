package connmgr

import (
	"context"
	"testing"
	"time"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	inet "gx/ipfs/QmXuRkCR7BNQa9uqfpTiFWsTQLzmTWYg91Ja1w95gnqb6u/go-libp2p-net"
	tu "gx/ipfs/Qma6ESRQTf1ZLPgzpCwDTqQJefPnU6uLvMjP18vK8EWp8L/go-testutil"
)

type tconn struct {
	inet.Conn
	peer   peer.ID
	closed bool
}

func (c *tconn) Close() error {
	c.closed = true
	return nil
}

func (c *tconn) RemotePeer() peer.ID {
	return c.peer
}

func (c *tconn) RemoteMultiaddr() ma.Multiaddr {
	addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234")
	if err != nil {
		panic("cannot create multiaddr")
	}
	return addr
}

func randConn(t *testing.T) inet.Conn {
	pid := tu.RandPeerIDFatal(t)
	return &tconn{peer: pid}
}

func TestConnTrimming(t *testing.T) {
	cm := NewConnManager(200, 300, 0)
	not := cm.Notifee()

	var conns []inet.Conn
	for i := 0; i < 300; i++ {
		rc := randConn(t)
		conns = append(conns, rc)
		not.Connected(nil, rc)
	}

	for _, c := range conns {
		if c.(*tconn).closed {
			t.Fatal("nothing should be closed yet")
		}
	}

	for i := 0; i < 100; i++ {
		cm.TagPeer(conns[i].RemotePeer(), "foo", 10)
	}

	cm.TagPeer(conns[299].RemotePeer(), "badfoo", -5)

	cm.TrimOpenConns(context.Background())

	for i := 0; i < 100; i++ {
		c := conns[i]
		if c.(*tconn).closed {
			t.Fatal("these shouldnt be closed")
		}
	}

	if !conns[299].(*tconn).closed {
		t.Fatal("conn with bad tag should have gotten closed")
	}
}

func TestConnsToClose(t *testing.T) {
	cm := NewConnManager(0, 10, 0)
	conns := cm.getConnsToClose(context.Background())
	if conns != nil {
		t.Fatal("expected no connections")
	}

	cm = NewConnManager(10, 0, 0)
	conns = cm.getConnsToClose(context.Background())
	if conns != nil {
		t.Fatal("expected no connections")
	}

	cm = NewConnManager(1, 1, 0)
	conns = cm.getConnsToClose(context.Background())
	if conns != nil {
		t.Fatal("expected no connections")
	}

	cm = NewConnManager(1, 1, time.Duration(10*time.Minute))
	not := cm.Notifee()
	for i := 0; i < 5; i++ {
		conn := randConn(t)
		not.Connected(nil, conn)
	}
	conns = cm.getConnsToClose(context.Background())
	if len(conns) != 0 {
		t.Fatal("expected no connections")
	}
}

func TestGetTagInfo(t *testing.T) {
	start := time.Now()
	cm := NewConnManager(1, 1, time.Duration(10*time.Minute))
	not := cm.Notifee()
	conn := randConn(t)
	not.Connected(nil, conn)
	end := time.Now()

	other := tu.RandPeerIDFatal(t)
	tag := cm.GetTagInfo(other)
	if tag != nil {
		t.Fatal("expected no tag")
	}

	tag = cm.GetTagInfo(conn.RemotePeer())
	if tag == nil {
		t.Fatal("expected tag")
	}
	if tag.FirstSeen.Before(start) || tag.FirstSeen.After(end) {
		t.Fatal("expected first seen time")
	}
	if tag.Value != 0 {
		t.Fatal("expected zero value")
	}
	if len(tag.Tags) != 0 {
		t.Fatal("expected no tags")
	}
	if len(tag.Conns) != 1 {
		t.Fatal("expected one connection")
	}
	for s, tm := range tag.Conns {
		if s != conn.RemoteMultiaddr().String() {
			t.Fatal("unexpected multiaddr")
		}
		if tm.Before(start) || tm.After(end) {
			t.Fatal("unexpected connection time")
		}
	}

	cm.TagPeer(conn.RemotePeer(), "tag", 5)
	tag = cm.GetTagInfo(conn.RemotePeer())
	if tag == nil {
		t.Fatal("expected tag")
	}
	if tag.FirstSeen.Before(start) || tag.FirstSeen.After(end) {
		t.Fatal("expected first seen time")
	}
	if tag.Value != 5 {
		t.Fatal("expected five value")
	}
	if len(tag.Tags) != 1 {
		t.Fatal("expected no tags")
	}
	for tString, v := range tag.Tags {
		if tString != "tag" || v != 5 {
			t.Fatal("expected tag value")
		}
	}
	if len(tag.Conns) != 1 {
		t.Fatal("expected one connection")
	}
	for s, tm := range tag.Conns {
		if s != conn.RemoteMultiaddr().String() {
			t.Fatal("unexpected multiaddr")
		}
		if tm.Before(start) || tm.After(end) {
			t.Fatal("unexpected connection time")
		}
	}
}

func TestTagPeerNonExistant(t *testing.T) {
	cm := NewConnManager(1, 1, time.Duration(10*time.Minute))

	id := tu.RandPeerIDFatal(t)
	cm.TagPeer(id, "test", 1)

	if len(cm.peers) != 0 {
		t.Fatal("expected zero peers")
	}
}

func TestUntagPeer(t *testing.T) {
	cm := NewConnManager(1, 1, time.Duration(10*time.Minute))
	not := cm.Notifee()
	conn := randConn(t)
	not.Connected(nil, conn)
	rp := conn.RemotePeer()
	cm.TagPeer(rp, "tag", 5)
	cm.TagPeer(rp, "tag two", 5)

	id := tu.RandPeerIDFatal(t)
	cm.UntagPeer(id, "test")
	if len(cm.peers[rp].tags) != 2 {
		t.Fatal("expected tags to be uneffected")
	}

	cm.UntagPeer(conn.RemotePeer(), "test")
	if len(cm.peers[rp].tags) != 2 {
		t.Fatal("expected tags to be uneffected")
	}

	cm.UntagPeer(conn.RemotePeer(), "tag")
	if len(cm.peers[rp].tags) != 1 {
		t.Fatal("expected tag to be removed")
	}
	if cm.peers[rp].value != 5 {
		t.Fatal("expected aggreagte tag value to be 5")
	}
}

func TestGetInfo(t *testing.T) {
	start := time.Now()
	gp := time.Duration(10 * time.Minute)
	cm := NewConnManager(1, 5, gp)
	not := cm.Notifee()
	conn := randConn(t)
	not.Connected(nil, conn)
	cm.TrimOpenConns(context.Background())
	end := time.Now()

	info := cm.GetInfo()
	if info.HighWater != 5 {
		t.Fatal("expected highwater to be 5")
	}
	if info.LowWater != 1 {
		t.Fatal("expected highwater to be 1")
	}
	if info.LastTrim.Before(start) || info.LastTrim.After(end) {
		t.Fatal("unexpected last trim time")
	}
	if info.GracePeriod != gp {
		t.Fatal("unexpected grace period")
	}
	if info.ConnCount != 1 {
		t.Fatal("unexpected number of connections")
	}
}

func TestDoubleConnection(t *testing.T) {
	gp := time.Duration(10 * time.Minute)
	cm := NewConnManager(1, 5, gp)
	not := cm.Notifee()
	conn := randConn(t)
	not.Connected(nil, conn)
	cm.TagPeer(conn.RemotePeer(), "foo", 10)
	not.Connected(nil, conn)
	if cm.connCount != 1 {
		t.Fatal("unexpected number of connections")
	}
	if cm.peers[conn.RemotePeer()].value != 10 {
		t.Fatal("unexpected peer value")
	}
}

func TestDisconnected(t *testing.T) {
	gp := time.Duration(10 * time.Minute)
	cm := NewConnManager(1, 5, gp)
	not := cm.Notifee()
	conn := randConn(t)
	not.Connected(nil, conn)
	cm.TagPeer(conn.RemotePeer(), "foo", 10)

	not.Disconnected(nil, randConn(t))
	if cm.connCount != 1 {
		t.Fatal("unexpected number of connections")
	}
	if cm.peers[conn.RemotePeer()].value != 10 {
		t.Fatal("unexpected peer value")
	}

	not.Disconnected(nil, &tconn{peer: conn.RemotePeer()})
	if cm.connCount != 1 {
		t.Fatal("unexpected number of connections")
	}
	if cm.peers[conn.RemotePeer()].value != 10 {
		t.Fatal("unexpected peer value")
	}

	not.Disconnected(nil, conn)
	if cm.connCount != 0 {
		t.Fatal("unexpected number of connections")
	}
	if len(cm.peers) != 0 {
		t.Fatal("unexpected number of peers")
	}
}
