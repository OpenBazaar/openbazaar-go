package ipfs

import (
	"context"
	"errors"

	ipath "gx/ipfs/QmQAgv6Gaoe2tQpcabqwKXKChp2MZ7i3UXv9DqTTaxCaTR/go-path"

	"github.com/ipfs/go-ipfs/core"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("ipfs")

var pubErr = errors.New(`name publish failed`)

// Publish a signed IPNS record to our Peer ID
func Publish(n *core.IpfsNode, hash string) error {
	err := n.Namesys.Publish(context.Background(), n.PrivateKey, ipath.FromString("/ipfs/"+hash))
	if err == nil {
		log.Infof("Published %s to IPNS", hash)
		return nil
	} else {
		return pubErr
	}
}
