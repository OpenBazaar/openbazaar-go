package main

import (
	"io"
	"os"

	"gx/ipfs/QmTsHcKgTQ4VeYZd8eKYpTXeLW7KNwkRD9wjnrwsV2sToq/go-colorable"
)

func main() {
	io.Copy(colorable.NewColorableStdout(), os.Stdin)
}
