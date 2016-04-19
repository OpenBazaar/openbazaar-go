package ipfs

import (
	"path"
	"errors"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core/coreunix"
)

var addErr = errors.New(`Add directory failed`)

// Resursively add a directory to IPFS and return the root hash
func AddDirectory(ctx commands.Context, fpath string) (string, error) {
	_, root := path.Split(fpath)
	args := []string{"add", "-r", fpath}
	req, cmd, err := NewRequest(ctx, args)
	if err != nil {
		return "", err
	}
	res := commands.NewResponse(req)
	cmd.PreRun(req)
	cmd.Run(req, res)
	var rootHash string
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
	args := []string{"add", fpath}
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