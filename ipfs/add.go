package ipfs

import (
	"context"
	"gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	"gx/ipfs/QmZMWMvWMVKCbHetJ4RgndbuEF1io2UpUxwQwtNjtYPzSC/go-ipfs-files"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreunix"
	_ "github.com/ipfs/go-ipfs/core/mock"
)

// Recursively add a directory to IPFS and return the root hash
func AddDirectory(n *core.IpfsNode, root string) (rootHash string, err error) {
	s := strings.Split(root, "/")
	dirName := s[len(s)-1]
	h, err := addAndPin(n, root)
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
	return addAndPin(n, file)
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

func addAndPin(n *core.IpfsNode, root string) (rootHash string, err error) {
	defer n.Blockstore.PinLock().Unlock()

	stat, err := os.Lstat(root)
	if err != nil {
		return "", err
	}

	f, err := files.NewSerialFile(filepath.Base(root), root, false, stat)
	if err != nil {
		return "", err
	}
	defer f.Close()

	fileAdder, err := coreunix.NewAdder(n.Context(), n.Pinning, n.Blockstore, n.DAG)
	if err != nil {
		return "", err
	}

	node, err := fileAdder.AddAllAndPin(f)
	if err != nil {
		return "", err
	}
	return node.Cid().String(), nil
}
