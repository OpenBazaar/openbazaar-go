package ipfs

import (
	"path"
	"errors"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core/coreunix"
	"github.com/op/go-logging"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

var log = logging.MustGetLogger("ipfs")

var addErr = errors.New(`Add directory failed`)

// Resursively add a directory to IPFS and return the root hash
func AddDirectory(ctx commands.Context, fpath string) (string, error) {
	_, root := path.Split(fpath)
	args := []string{"add", "-r", fpath}
	req, cmd, err := NewRequest(args)
	if err != nil {
		return "", err
	}
	res := commands.NewResponse(req)
	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req.SetRootContext(cctx)
	req.SetInvocContext(ctx)
	if err != nil {
		return "", err
	}
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
	log.Infof("Added directory %s to IPFS", rootHash)
	return rootHash, nil
}