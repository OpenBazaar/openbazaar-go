package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/OpenBazaar/openbazaar-go/mobile"
	"github.com/jessevdk/go-flags"
)

type Options struct {
	Datadir string `short:"d" long:"data" description:"location of openbazaar datastore"`
}

var (
	options Options
	parser  = flags.NewParser(&options, flags.Default)
)

func main() {
	var dataPath = "/Users/mg/work/ob/openbazaar-go/config_mobile_test"
	if _, err := parser.Parse(); err != nil {
		if len(os.Args) > 1 && os.Args[1] == "-h" {
			os.Exit(0)
		}
		fmt.Printf("error parsing options: %s\n", err.Error())
		os.Exit(1)
	} else {
		if options.Datadir != "" {
			dataPath = options.Datadir
		}
	}

	var (
		wg     sync.WaitGroup
		n, err = mobile.NewNodeWithConfig(&mobile.NodeConfig{
			RepoPath: dataPath,
		}, "", "")
	)
	if err != nil {
		fmt.Println(err.Error())
	}
	wg.Add(1)
	if err := n.Start(); err != nil {
		fmt.Println(err.Error())
	}
	wg.Wait()
}
