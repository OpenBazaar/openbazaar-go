package ipfs

import (
	"context"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreunix"
	"github.com/ipfs/go-ipfs/path"
	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"io/ioutil"
	"strings"
	"time"
)

// Fetch data from IPFS given the hash
func Cat(n *core.IpfsNode, path string, timeout time.Duration) ([]byte, error) {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	if !strings.HasPrefix(path, "/ipfs/") {
		path = "/ipfs/" + path
	}

	r, err := coreunix.Cat(ctx, n, path)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(r)
}

func ResolveThenCat(n *core.IpfsNode, ipnsPath path.Path, timeout time.Duration, usecache bool) ([]byte, error) {
	var ret []byte
	pid, err := peer.IDB58Decode(ipnsPath.Segments()[0])
	if err != nil {
		return nil, err
	}
	hash, err := Resolve(n, pid, timeout, usecache)
	if err != nil {
		return ret, err
	}
	p := make([]string, len(ipnsPath.Segments()))
	p[0] = hash
	for i := 0; i < len(ipnsPath.Segments())-1; i++ {
		p[i+1] = ipnsPath.Segments()[i+1]
	}
	b, err := Cat(n, path.Join(p), timeout)
	if err != nil {
		return ret, err
	}
	return b, nil
}
