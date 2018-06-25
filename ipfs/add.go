package ipfs

import (
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreunix"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
)

// Resursively add a directory to IPFS and return the root hash
func AddDirectory(n *core.IpfsNode, root string) (rootHash string, err error) {
	return coreunix.AddR(n, root)
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
