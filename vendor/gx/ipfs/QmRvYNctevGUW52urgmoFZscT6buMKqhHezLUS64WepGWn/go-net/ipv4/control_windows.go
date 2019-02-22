// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ipv4

import (
	"syscall"

	"gx/ipfs/QmRvYNctevGUW52urgmoFZscT6buMKqhHezLUS64WepGWn/go-net/internal/socket"
)

func setControlMessage(c *socket.Conn, opt *rawOpt, cf ControlFlags, on bool) error {
	// TODO(mikio): implement this
	return syscall.EWINDOWS
}
