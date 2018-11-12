package main

import (
	"fmt"
	"os"
	"strings"

	cidutil "gx/ipfs/QmQJSeE3CX4zos9qeaG8EhecEK9zvrTEfTG84J8C5NVRwt/go-cidutil"

	c "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mb "gx/ipfs/QmekxXDhCxCJRNuzmHreuaT3BsuJcsjcXWNrtV9C8DRHtd/go-multibase"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s [-b multibase-code] [-v cid-version] <fmt-str> <cid> ...\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "<fmt-str> is either 'prefix' or a printf style format string:\n%s", cidutil.FormatRef)
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	newBase := mb.Encoding(-1)
	var verConv func(cid c.Cid) (c.Cid, error)
	args := os.Args[1:]
outer:
	for {
		switch args[0] {
		case "-b":
			if len(args) < 2 {
				usage()
			}
			encoder, err := mb.EncoderByName(args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
				os.Exit(2)
			}
			newBase = encoder.Encoding()
			args = args[2:]
		case "-v":
			if len(args) < 2 {
				usage()
			}
			switch args[1] {
			case "0":
				verConv = toCidV0
			case "1":
				verConv = toCidV1
			default:
				fmt.Fprintf(os.Stderr, "Error: Invalid cid version: %s\n", args[1])
				os.Exit(2)
			}
			args = args[2:]
		default:
			break outer
		}
	}
	if len(args) < 2 {
		usage()
	}
	fmtStr := args[0]
	switch fmtStr {
	case "prefix":
		fmtStr = "%P"
	default:
		if strings.IndexByte(fmtStr, '%') == -1 {
			fmt.Fprintf(os.Stderr, "Error: Invalid format string: %s\n", fmtStr)
			os.Exit(2)
		}
	}
	for _, cidStr := range args[1:] {
		cid, err := c.Decode(cidStr)
		if err != nil {
			fmt.Fprintf(os.Stdout, "!INVALID_CID!\n")
			errorMsg("%s: %v", cidStr, err)
			// Don't abort on a bad cid
			continue
		}
		base := newBase
		if newBase == -1 {
			base, _ = c.ExtractEncoding(cidStr)
		}
		if verConv != nil {
			cid, err = verConv(cid)
			if err != nil {
				fmt.Fprintf(os.Stdout, "!ERROR!\n")
				errorMsg("%s: %v", cidStr, err)
				// Don't abort on a bad conversion
				continue
			}
		}
		str, err := cidutil.Format(fmtStr, base, cid)
		switch err.(type) {
		case cidutil.FormatStringError:
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(2)
		default:
			fmt.Fprintf(os.Stdout, "!ERROR!\n")
			errorMsg("%s: %v", cidStr, err)
			// Don't abort on cid specific errors
			continue
		case nil:
			// no error
		}
		fmt.Fprintf(os.Stdout, "%s\n", str)
	}
	os.Exit(exitCode)
}

var exitCode = 0

func errorMsg(fmtStr string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: ")
	fmt.Fprintf(os.Stderr, fmtStr, a...)
	fmt.Fprintf(os.Stderr, "\n")
	exitCode = 1
}

func toCidV0(cid c.Cid) (c.Cid, error) {
	if cid.Type() != c.DagProtobuf {
		return c.Cid{}, fmt.Errorf("can't convert non-protobuf nodes to cidv0")
	}
	return c.NewCidV0(cid.Hash()), nil
}

func toCidV1(cid c.Cid) (c.Cid, error) {
	return c.NewCidV1(cid.Type(), cid.Hash()), nil
}
