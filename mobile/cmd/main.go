package main

import (
	"fmt"
	"github.com/OpenBazaar/openbazaar-go/mobile"
	"github.com/jessevdk/go-flags"
	"os"
	"path"
)

type Options struct {
	TestnetEnabled bool   `short:"t" long:"testnet" description:"use testnet protocol and wallet endpoints"`
	Datadir        string `short:"d" long:"data" description:"location of openbazaar datastore"`
}

var (
	options Options
	parser  = flags.NewParser(&options, flags.Default)
)

func main() {
	var dataPath = path.Join(os.TempDir(), "ob-mobile")
	if _, err := parser.Parse(); err != nil {
		if len(os.Args) > 1 && os.Args[1] == "-h" {
			os.Exit(0)
		}
		fmt.Printf("error parsing options: %s\n", err.Error())
		os.Exit(1)
	}

	if options.Datadir != "" {
		dataPath = options.Datadir
	}

	var (
		n, err = mobile.NewNodeWithConfig(&mobile.NodeConfig{
			RepoPath: dataPath,
			Testnet:  options.TestnetEnabled,
		}, "", "")
	)
	if err != nil {
		fmt.Println(err.Error())
	}
	if err := n.Start(); err != nil {
		fmt.Println(err.Error())
	}

	select {}
}
