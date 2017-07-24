package ipfs

import (
	"errors"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core/coreunix"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"strconv"
)

var addErr = errors.New(`Add directory failed`)

// Resursively add a directory to IPFS and return the root hash
func AddDirectory(ctx commands.Context, fpath string) (rootHash string, err error) {
	_, root := path.Split(fpath)
	args := []string{"add", "-r", "--cid-version", strconv.Itoa(1), fpath}
	req, cmd, err := NewRequest(ctx, args)
	if err != nil {
		return "", err
	}
	res := commands.NewResponse(req)
	cmd.PreRun(req)
	cmd.Run(req, res)
	for r := range res.Output().(<-chan interface{}) {
		if r.(*coreunix.AddedObject).Name == root {
			rootHash = r.(*coreunix.AddedObject).Hash
		}
	}
	cmd.PostRun(req, res)
	if res.Error() != nil {
		return "", res.Error()
	}
	if rootHash == "" {
		return "", addErr
	}
	return rootHash, nil
}

func AddFile(ctx commands.Context, fpath string) (string, error) {
	args := []string{"add", "--cid-version", strconv.Itoa(1), fpath}
	req, cmd, err := NewRequest(ctx, args)
	if err != nil {
		return "", err
	}
	res := commands.NewResponse(req)
	cmd.PreRun(req)
	cmd.Run(req, res)
	var fileHash string
	for r := range res.Output().(<-chan interface{}) {
		fileHash = r.(*coreunix.AddedObject).Hash
	}
	cmd.PostRun(req, res)
	if res.Error() != nil {
		return "", res.Error()
	}
	if fileHash == "" {
		return "", addErr
	}
	return fileHash, nil
}

func GetHashOfFile(ctx commands.Context, fpath string) (string, error) {
	args := []string{"add", "-n", "--cid-version", strconv.Itoa(1), fpath}
	req, cmd, err := NewRequest(ctx, args)
	if err != nil {
		return "", err
	}
	res := commands.NewResponse(req)
	cmd.PreRun(req)
	cmd.Run(req, res)
	var fileHash string
	for r := range res.Output().(<-chan interface{}) {
		fileHash = r.(*coreunix.AddedObject).Hash
	}
	cmd.PostRun(req, res)
	if res.Error() != nil {
		return "", res.Error()
	}
	if fileHash == "" {
		return "", addErr
	}
	return fileHash, nil
}

func GetHash(ctx commands.Context, reader io.Reader) (string, error) {
	tmpPath := path.Join(ctx.ConfigRoot, strconv.Itoa(rand.Int()))
	f, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}
	f.Write(b)
	defer f.Close()
	defer os.Remove(tmpPath)
	return GetHashOfFile(ctx, tmpPath)
}
