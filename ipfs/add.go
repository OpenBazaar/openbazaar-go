package ipfs

import (
	"context"
	"gx/ipfs/QmQmhotPUzVrMEWNK3x1R5jQ5ZHWyL7tVUrmRPjrBrvyCb/go-ipfs-files"
	"gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core/options"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/coreunix"
	_ "github.com/ipfs/go-ipfs/core/mock"
)

// Recursively add a directory to IPFS and return the root hash
func AddDirectory(n *core.IpfsNode, root string) (rootHash string, err error) {
	api, err := coreapi.NewCoreAPI(n)
	if err != nil {
		return "", err
	}
	stat, err := os.Lstat(root)
	if err != nil {
		return "", err
	}

	f, err := files.NewSerialFile(root, false, stat)
	if err != nil {
		return "", err
	}

	opts := []options.UnixfsAddOption{
		options.Unixfs.CidVersion(0),
		options.Unixfs.Pin(true),
		options.Unixfs.Wrap(true),
	}
	pth, err := api.Unixfs().Add(context.Background(), files.ToDir(f), opts...)
	if err != nil {
		return "", err
	}
	return pth.Root().String(), nil
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

	f, err := files.NewSerialFile(root, false, stat)
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
