package ipfs

import (
	"context"
	ipath "gx/ipfs/QmT3rzed1ppXefourpmoZ7tyVQfsGPQZ1pHDngLmCvXxd3/go-path"
	"gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	"io/ioutil"
	"strings"
	"time"

	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/coreapi/interface"

	"github.com/ipfs/go-ipfs/core"
)

// Fetch data from IPFS given the hash
func Cat(n *core.IpfsNode, path string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if !strings.HasPrefix(path, "/ipfs/") {
		path = "/ipfs/" + path
	}
	api := coreapi.NewCoreAPI(n)
	pth, err := iface.ParsePath(path)
	if err != nil {
		return nil, err
	}

	r, err := api.Unixfs().Get(ctx, pth)
	if err != nil {
		return nil, err
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
