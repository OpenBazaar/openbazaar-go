// +build !windows,!darwin,!linux,!netbsd,!openbsd
package reuseport

import (
	"syscall"
)

func Control(network, address string, c syscall.RawConn) error {
	return nil
}
