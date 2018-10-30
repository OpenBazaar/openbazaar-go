package ipfs

import (
	"context"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreunix"
	_ "github.com/ipfs/go-ipfs/core/mock"
	"github.com/ipfs/go-ipfs/merkledag"
)

// Recursively add a directory to IPFS and return the root hash
func AddDirectory(n *core.IpfsNode, root string) (rootHash string, err error) {
	s := strings.Split(root, "/")
	dirName := s[len(s)-1]
	h, err := coreunix.AddR(n, root)
	if err != nil {
		return "", err
	}
	i, err := cid.Decode(h)
	if err != nil {
		return "", err
	}
	dag := merkledag.NewDAGService(n.Blocks)
	m := make(map[string]bool)
	ctx := context.Background()
	m[i.String()] = true
	for {
		if len(m) == 0 {
			break
		}
		for k := range m {
			c, err := cid.Decode(k)
			if err != nil {
				return "", err
			}
			links, err := dag.GetLinks(ctx, c)
			if err != nil {
				return "", err
			}
			delete(m, k)
			for _, link := range links {
				if link.Name == dirName {
					return link.Cid.String(), nil
				}
				m[link.Cid.String()] = true
			}
		}
	}
	return i.String(), nil
}

func AddFile(n *core.IpfsNode, file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", nil
	}
	return coreunix.Add(n, f)
}

func GetHashOfFile(n *core.IpfsNode, fpath string) (string, error) {
	return AddFile(n, fpath)
}

func GetHash(n *core.IpfsNode, reader io.Reader) (string, error) {
	f, err := ioutil.TempFile("", strconv.Itoa(rand.Int()))
	if err != nil {
		return "", err
	}
	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}
	f.Write(b)
	defer f.Close()
	return GetHashOfFile(n, f.Name())
}
