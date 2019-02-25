// +build !windows
// +build !appengine

package colorable

import (
	"io"
	"os"

	_ "gx/ipfs/QmWVGFCGGTAGrT3adV261k1h6nntcyDhGVszGV2i2pzwPe/go-isatty"
)

// NewColorable return new instance of Writer which handle escape sequence.
func NewColorable(file *os.File) io.Writer {
	if file == nil {
		panic("nil passed instead of *os.File to NewColorable()")
	}

	return file
}

// NewColorableStdout return new instance of Writer which handle escape sequence for stdout.
func NewColorableStdout() io.Writer {
	return os.Stdout
}

// NewColorableStderr return new instance of Writer which handle escape sequence for stderr.
func NewColorableStderr() io.Writer {
	return os.Stderr
}
