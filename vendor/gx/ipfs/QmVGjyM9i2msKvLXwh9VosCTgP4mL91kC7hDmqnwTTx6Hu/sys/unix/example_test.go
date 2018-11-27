// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package unix_test

import (
	"log"
	"os"

	"gx/ipfs/QmVGjyM9i2msKvLXwh9VosCTgP4mL91kC7hDmqnwTTx6Hu/sys/unix"
)

func ExampleExec() {
	err := unix.Exec("/bin/ls", []string{"ls", "-al"}, os.Environ())
	log.Fatal(err)
}
