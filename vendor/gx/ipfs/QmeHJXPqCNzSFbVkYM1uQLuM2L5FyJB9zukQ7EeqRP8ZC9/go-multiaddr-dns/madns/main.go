package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	madns "gx/ipfs/QmeHJXPqCNzSFbVkYM1uQLuM2L5FyJB9zukQ7EeqRP8ZC9/go-multiaddr-dns"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Print("usage: madns /dnsaddr/example.com\n" +
			"       madns /dnsaddr/example.com/ipfs/Qmfoobar\n" +
			"       madns /dns6/example.com\n" +
			"       madns /dns6/example.com/tcp/443/wss\n" +
			"       madns /dns4/example.com\n")
		os.Exit(1)
	}

	query := os.Args[1]
	if !strings.HasPrefix(query, "/") {
		query = "/dnsaddr/" + query
		fmt.Fprintf(os.Stderr, "madns: changing query to %s\n", query)
	}

	maddr, err := ma.NewMultiaddr(query)
	if err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}

	rmaddrs, err := madns.Resolve(context.Background(), maddr)
	if err != nil {
		fmt.Printf("error: %s (result=%+v)\n", err, rmaddrs)
		os.Exit(1)
	}

	for _, r := range rmaddrs {
		fmt.Println(r.String())
	}
}
