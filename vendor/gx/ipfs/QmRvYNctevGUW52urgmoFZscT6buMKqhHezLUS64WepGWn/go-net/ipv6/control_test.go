// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ipv6_test

import (
	"testing"

	"gx/ipfs/QmRvYNctevGUW52urgmoFZscT6buMKqhHezLUS64WepGWn/go-net/ipv6"
)

func TestControlMessageParseWithFuzz(t *testing.T) {
	var cm ipv6.ControlMessage
	for _, fuzz := range []string{
		"\f\x00\x00\x00)\x00\x00\x00.\x00\x00\x00",
		"\f\x00\x00\x00)\x00\x00\x00,\x00\x00\x00",
	} {
		cm.Parse([]byte(fuzz))
	}
}
