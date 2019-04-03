package ipfs

import (
	"context"
	ipath "gx/ipfs/QmQAgv6Gaoe2tQpcabqwKXKChp2MZ7i3UXv9DqTTaxCaTR/go-path"
	"gx/ipfs/QmQmhotPUzVrMEWNK3x1R5jQ5ZHWyL7tVUrmRPjrBrvyCb/go-ipfs-files"
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	"io/ioutil"
	"strings"
	"time"

	"github.com/go-errors/errors"

	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"

	"github.com/ipfs/go-ipfs/core/coreapi"

	"github.com/ipfs/go-ipfs/core"
)

// Fetch data from IPFS given the hash
func Cat(n *core.IpfsNode, path string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if !strings.HasPrefix(path, "/ipfs/") {
		path = "/ipfs/" + path
	}
	api, err := coreapi.NewCoreAPI(n)
	if err != nil {
		return nil, err
	}
	pth, err := coreiface.ParsePath(path)
	if err != nil {
		return nil, err
	}

	nd, err := api.Unixfs().Get(ctx, pth)
	if err != nil {
		return nil, err
	}

	r, ok := nd.(files.File)
	if !ok {
		return nil, errors.New("Received incorrect type from Unixfs().Get()")
	}

	return ioutil.ReadAll(r)
}

func ResolveThenCat(n *core.IpfsNode, ipnsPath ipath.Path, timeout time.Duration, quorum uint, usecache bool) ([]byte, error) {
	var ret []byte
	pid, err := peer.IDB58Decode(ipnsPath.Segments()[0])
	if err != nil {
		return nil, err
	}
	hash, err := Resolve(n, pid, timeout, quorum, usecache)
	if err != nil {
		return ret, err
	}
	p := make([]string, len(ipnsPath.Segments()))
	p[0] = hash
	for i := 0; i < len(ipnsPath.Segments())-1; i++ {
		p[i+1] = ipnsPath.Segments()[i+1]
	}
	b, err := Cat(n, ipath.Join(p), timeout)
	if err != nil {
		return ret, err
	}
	return b, nil
}
