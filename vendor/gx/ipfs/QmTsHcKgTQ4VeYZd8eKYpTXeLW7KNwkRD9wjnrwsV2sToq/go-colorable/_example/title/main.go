package main

import (
	"fmt"
	. "gx/ipfs/QmTsHcKgTQ4VeYZd8eKYpTXeLW7KNwkRD9wjnrwsV2sToq/go-colorable"
	"os"
)

func main() {
	out := NewColorableStdout()
	fmt.Fprint(out, "\x1B]0;TITLE Changed\007(See title and hit any key)")
	var c [1]byte
	os.Stdin.Read(c[:])
}
