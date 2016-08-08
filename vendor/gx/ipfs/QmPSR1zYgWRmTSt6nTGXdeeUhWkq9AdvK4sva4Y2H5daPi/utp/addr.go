package utp

import "net"

type Addr struct {
	Child net.Addr
}

func (me *Addr) Network() string {
	return "utp"
}

func (me *Addr) String() string {
	return me.Child.String()
}
