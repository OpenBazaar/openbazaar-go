package net

import (
	"testing"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
)

func TestListen(T *testing.T) {
	var notifee NotifyBundle
	addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234")
	if err != nil {
		T.Fatal("unexpected multiaddr error")
	}
	notifee.Listen(nil, addr)

	called := false
	notifee.ListenF = func(Network, ma.Multiaddr) {
		called = true
	}
	if called {
		T.Fatal("called should be false")
	}

	notifee.Listen(nil, addr)
	if !called {
		T.Fatal("Listen should have been called")
	}
}

func TestListenClose(T *testing.T) {
	var notifee NotifyBundle
	addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234")
	if err != nil {
		T.Fatal("unexpected multiaddr error")
	}
	notifee.ListenClose(nil, addr)

	called := false
	notifee.ListenCloseF = func(Network, ma.Multiaddr) {
		called = true
	}
	if called {
		T.Fatal("called should be false")
	}

	notifee.ListenClose(nil, addr)
	if !called {
		T.Fatal("ListenClose should have been called")
	}
}

func TestConnected(T *testing.T) {
	var notifee NotifyBundle
	notifee.Connected(nil, nil)

	called := false
	notifee.ConnectedF = func(Network, Conn) {
		called = true
	}
	if called {
		T.Fatal("called should be false")
	}

	notifee.Connected(nil, nil)
	if !called {
		T.Fatal("Connected should have been called")
	}
}

func TestDisconnected(T *testing.T) {
	var notifee NotifyBundle
	notifee.Disconnected(nil, nil)

	called := false
	notifee.DisconnectedF = func(Network, Conn) {
		called = true
	}
	if called {
		T.Fatal("called should be false")
	}

	notifee.Disconnected(nil, nil)
	if !called {
		T.Fatal("Disconnected should have been called")
	}
}

func TestOpenedStream(T *testing.T) {
	var notifee NotifyBundle
	notifee.OpenedStream(nil, nil)

	called := false
	notifee.OpenedStreamF = func(Network, Stream) {
		called = true
	}
	if called {
		T.Fatal("called should be false")
	}

	notifee.OpenedStream(nil, nil)
	if !called {
		T.Fatal("OpenedStream should have been called")
	}
}

func TestClosedStream(T *testing.T) {
	var notifee NotifyBundle
	notifee.ClosedStream(nil, nil)

	called := false
	notifee.ClosedStreamF = func(Network, Stream) {
		called = true
	}
	if called {
		T.Fatal("called should be false")
	}

	notifee.ClosedStream(nil, nil)
	if !called {
		T.Fatal("ClosedStream should have been called")
	}
}
