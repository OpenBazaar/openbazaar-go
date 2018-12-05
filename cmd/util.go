package cmd

import (
	"net"
)

// DummyWriter - mock writer
type DummyWriter struct{}

// Write - pretend to write
func (d *DummyWriter) Write(p []byte) (n int, err error) {
	return 0, nil
}

// DummyListener - mock listener
type DummyListener struct {
	addr net.Addr
}

// Addr - return addr
func (d *DummyListener) Addr() net.Addr {
	return d.addr
}

// Accept - open a fileconn
func (d *DummyListener) Accept() (net.Conn, error) {
	conn, _ := net.FileConn(nil)
	return conn, nil
}

// Close - stop listener
func (d *DummyListener) Close() error {
	return nil
}
