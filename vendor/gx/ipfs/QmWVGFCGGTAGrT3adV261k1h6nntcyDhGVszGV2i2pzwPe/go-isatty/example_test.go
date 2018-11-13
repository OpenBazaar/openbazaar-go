package isatty_test

import (
	"fmt"
	"os"

	"gx/ipfs/QmWVGFCGGTAGrT3adV261k1h6nntcyDhGVszGV2i2pzwPe/go-isatty"
)

func Example() {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Println("Is Terminal")
	} else if isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		fmt.Println("Is Cygwin/MSYS2 Terminal")
	} else {
		fmt.Println("Is Not Terminal")
	}
}
