package ipfs

import (
	"errors"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core/coreunix"
	"github.com/ipfs/go-ipfs/importer/chunk"
	h "github.com/ipfs/go-ipfs/importer/helpers"
	ihelper "github.com/ipfs/go-ipfs/importer/helpers"
	"io"
	"path"
)

var addErr = errors.New(`Add directory failed`)

// Resursively add a directory to IPFS and return the root hash
func AddDirectory(ctx commands.Context, fpath string) (rootHash string, err error) {
	_, root := path.Split(fpath)
	args := []string{"add", "-r", fpath}
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

func GetHashOfFile(ctx commands.Context, fpath string) (string, error) {
	args := []string{"add", "-n", fpath}
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
	nd, err := ctx.ConstructNode()
	if err != nil {
		return "", err
	}
	chnk, err := chunk.FromString(reader, "")
	if err != nil {
		return "", err
	}
	params := ihelper.DagBuilderParams{
		Maxlinks: ihelper.DefaultLinksPerBlock,
		Dagserv:  nd.DAG,
	}
	db := params.New(chnk)

	var offset uint64 = 0
	var root *h.UnixfsNode
	for level := 0; !db.Done(); level++ {

		nroot := db.NewUnixfsNode()
		db.SetPosInfo(nroot, 0)

		// add our old root as a child of the new root.
		if root != nil { // nil if it's the first node.
			if err := nroot.AddChild(root, db); err != nil {
				return "", err
			}
		}

		// fill it up.
		if err := fillNodeRec(db, nroot, level, offset); err != nil {
			return "", err
		}

		offset = nroot.FileSize()
		root = nroot

	}
	if root == nil {
		root = db.NewUnixfsNode()
	}
	n, err := root.GetDagNode()
	if err != nil {
		return "", err
	}
	return n.String(), nil
}

// fillNodeRec will fill the given node with data from the dagBuilders input
// source down to an indirection depth as specified by 'depth'
// it returns the total dataSize of the node, and a potential error
//
// warning: **children** pinned indirectly, but input node IS NOT pinned.
func fillNodeRec(db *h.DagBuilderHelper, node *h.UnixfsNode, depth int, offset uint64) error {
	if depth < 0 {
		return errors.New("attempt to fillNode at depth < 0")
	}

	// Base case
	if depth <= 0 { // catch accidental -1's in case error above is removed.
		child, err := db.GetNextDataNode()
		if err != nil {
			return err
		}

		node.Set(child)
		return nil
	}

	// while we have room AND we're not done
	for node.NumChildren() < db.Maxlinks() && !db.Done() {
		child := db.NewUnixfsNode()
		db.SetPosInfo(child, offset)

		err := fillNodeRec(db, child, depth-1, offset)
		if err != nil {
			return err
		}

		if err := node.AddChild(child, db); err != nil {
			return err
		}
		offset += child.FileSize()
	}

	return nil
}
