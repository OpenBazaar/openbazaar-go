package meterconn

import (
	metrics "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/metrics"
	transport "gx/ipfs/QmWzfrG1PUeF8mDpYfNsRL3wh5Rkgnp68LAWUB2bhuDWRL/go-libp2p-transport"
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
