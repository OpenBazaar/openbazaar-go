package net

import (
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/corenet"
	"fmt"
)

var SERVICE_NAME string = "/app/openbazaar"


func SetupOpenBazaarService(node *core.IpfsNode) error {
	list, err := corenet.Listen(node, SERVICE_NAME)
	if err != nil {
		return err
	}

	go func() {
		for {
			con, err := list.Accept()
			if err != nil {
				log.Error(err)
			}

			defer con.Close()

			// Send hello world and exit for now. TODO: build handler
			fmt.Fprintln(con, "Hello World!")
			log.Infof("Connection from: %s\n", con.Conn().RemotePeer())
		}
	}()

	log.Infof("OpenBazaar service running at %s", SERVICE_NAME)
	return nil
}