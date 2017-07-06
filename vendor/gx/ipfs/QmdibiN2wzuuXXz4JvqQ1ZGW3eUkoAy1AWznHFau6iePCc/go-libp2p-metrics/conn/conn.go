package meterconn

import (
	transport "gx/ipfs/QmSKoeo64CA54WNWqzVXHi5aT2LbgzP7B9VzQjW5yh7d4H/go-libp2p-transport"
	metrics "gx/ipfs/QmdibiN2wzuuXXz4JvqQ1ZGW3eUkoAy1AWznHFau6iePCc/go-libp2p-metrics"
)

type MeteredConn struct {
	mesRecv metrics.MeterCallback
	mesSent metrics.MeterCallback

	transport.Conn
}

func WrapConn(bwc metrics.Reporter, c transport.Conn) transport.Conn {
	return newMeteredConn(c, bwc.LogRecvMessage, bwc.LogSentMessage)
}

func newMeteredConn(base transport.Conn, rcb metrics.MeterCallback, scb metrics.MeterCallback) transport.Conn {
	return &MeteredConn{
		Conn:    base,
		mesRecv: rcb,
		mesSent: scb,
	}
}

func (mc *MeteredConn) Read(b []byte) (int, error) {
	n, err := mc.Conn.Read(b)

	mc.mesRecv(int64(n))
	return n, err
}

func (mc *MeteredConn) Write(b []byte) (int, error) {
	n, err := mc.Conn.Write(b)

	mc.mesSent(int64(n))
	return n, err
}
