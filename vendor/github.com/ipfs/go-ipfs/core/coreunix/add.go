package coreunix

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	gopath "path"
	"path/filepath"
	"strconv"

	core "github.com/ipfs/go-ipfs/core"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/pin"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	posinfo "gx/ipfs/QmQyUyYcpKG1u53V7N25qRTGw5XwaAxTMKXbduqHotQztg/go-ipfs-posinfo"
	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
	dag "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag"
	chunker "gx/ipfs/QmTUTG9Jg9ZRA1EzTPGTDvnwfcfKhDMnqANnP9fe4rSjMR/go-ipfs-chunker"
	mfs "gx/ipfs/QmUwXQs8aZ472DmXZ8uJNf7HJNKoMJQVa7RaCz7ujZ3ua9/go-mfs"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
	files "gx/ipfs/QmZMWMvWMVKCbHetJ4RgndbuEF1io2UpUxwQwtNjtYPzSC/go-ipfs-files"
	bstore "gx/ipfs/QmcDDgAXDbpDUpadCJKLr49KYR4HuL7T8Z1dZTHt6ixsoR/go-ipfs-blockstore"
	unixfs "gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs"
	balanced "gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs/importer/balanced"
	ihelper "gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs/importer/helpers"
	trickle "gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs/importer/trickle"
)

var log = logging.Logger("coreunix")

// how many bytes of progress to wait before sending a progress update message
const progressReaderIncrement = 1024 * 256

var liveCacheSize = uint64(256 << 10)

type Link struct {
	Name, Hash string
	Size       uint64
}

type Object struct {
	Hash  string
	Links []Link
	Size  string
}

// NewAdder Returns a new Adder used for a file add operation.
func NewAdder(ctx context.Context, p pin.Pinner, bs bstore.GCBlockstore, ds ipld.DAGService) (*Adder, error) {
	bufferedDS := ipld.NewBufferedDAG(ctx, ds)

	return &Adder{
		ctx:        ctx,
		pinning:    p,
		blockstore: bs,
		dagService: ds,
		bufferedDS: bufferedDS,
		Progress:   false,
		Hidden:     true,
		Pin:        true,
		Trickle:    false,
		Wrap:       false,
		Chunker:    "",
	}, nil
}

// Adder holds the switches passed to the `add` command.
type Adder struct {
	ctx        context.Context
	pinning    pin.Pinner
	blockstore bstore.GCBlockstore
	dagService ipld.DAGService
	bufferedDS *ipld.BufferedDAG
	Out        chan<- interface{}
	Progress   bool
	Hidden     bool
	Pin        bool
	Trickle    bool
	RawLeaves  bool
	Silent     bool
	Wrap       bool
	Name       string
	NoCopy     bool
	Chunker    string
	root       ipld.Node
	mroot      *mfs.Root
	unlocker   bstore.Unlocker
	tempRoot   cid.Cid
	CidBuilder cid.Builder
	liveNodes  uint64
}

func (adder *Adder) mfsRoot() (*mfs.Root, error) {
	if adder.mroot != nil {
		return adder.mroot, nil
	}
	rnode := unixfs.EmptyDirNode()
	rnode.SetCidBuilder(adder.CidBuilder)
	mr, err := mfs.NewRoot(adder.ctx, adder.dagService, rnode, nil)
	if err != nil {
		return nil, err
	}
	adder.mroot = mr
	return adder.mroot, nil
}

// SetMfsRoot sets `r` as the root for Adder.
func (adder *Adder) SetMfsRoot(r *mfs.Root) {
	adder.mroot = r
}

// Constructs a node from reader's data, and adds it. Doesn't pin.
func (adder *Adder) add(reader io.Reader) (ipld.Node, error) {
	chnk, err := chunker.FromString(reader, adder.Chunker)
	if err != nil {
		return nil, err
	}

	// Make sure all added nodes are written when done.
	defer adder.bufferedDS.Commit()

	params := ihelper.DagBuilderParams{
		Dagserv:    adder.bufferedDS,
		RawLeaves:  adder.RawLeaves,
		Maxlinks:   ihelper.DefaultLinksPerBlock,
		NoCopy:     adder.NoCopy,
		CidBuilder: adder.CidBuilder,
	}

	if adder.Trickle {
		return trickle.Layout(params.New(chnk))
	}

	return balanced.Layout(params.New(chnk))
}

// RootNode returns the root node of the Added.
func (adder *Adder) RootNode() (ipld.Node, error) {
	// for memoizing
	if adder.root != nil {
		return adder.root, nil
	}

	mr, err := adder.mfsRoot()
	if err != nil {
		return nil, err
	}
	root, err := mr.GetDirectory().GetNode()
	if err != nil {
		return nil, err
	}

	// if not wrapping, AND one root file, use that hash as root.
	if !adder.Wrap && len(root.Links()) == 1 {
		nd, err := root.Links()[0].GetNode(adder.ctx, adder.dagService)
		if err != nil {
			return nil, err
		}

		root = nd
	}

	adder.root = root
	return root, err
}

// Recursively pins the root node of Adder and
// writes the pin state to the backing datastore.
func (adder *Adder) PinRoot() error {
	root, err := adder.RootNode()
	if err != nil {
		return err
	}
	if !adder.Pin {
		return nil
	}

	rnk := root.Cid()

	err = adder.dagService.Add(adder.ctx, root)
	if err != nil {
		return err
	}

	if adder.tempRoot.Defined() {
		err := adder.pinning.Unpin(adder.ctx, adder.tempRoot, true)
		if err != nil {
			return err
		}
		adder.tempRoot = rnk
	}

	adder.pinning.PinWithMode(rnk, pin.Recursive)
	return adder.pinning.Flush()
}

// Finalize flushes the mfs root directory and returns the mfs root node.
func (adder *Adder) Finalize() (ipld.Node, error) {
	mr, err := adder.mfsRoot()
	if err != nil {
		return nil, err
	}
	var root mfs.FSNode
	rootdir := mr.GetDirectory()
	root = rootdir

	err = root.Flush()
	if err != nil {
		return nil, err
	}

	var name string
	if !adder.Wrap {
		children, err := rootdir.ListNames(adder.ctx)
		if err != nil {
			return nil, err
		}

		if len(children) == 0 {
			return nil, fmt.Errorf("expected at least one child dir, got none")
		}

		// Replace root with the first child
		name = children[0]
		root, err = rootdir.Child(name)
		if err != nil {
			return nil, err
		}
	}

	err = adder.outputDirs(name, root)
	if err != nil {
		return nil, err
	}

	err = mr.Close()
	if err != nil {
		return nil, err
	}

	return root.GetNode()
}

func (adder *Adder) outputDirs(path string, fsn mfs.FSNode) error {
	switch fsn := fsn.(type) {
	case *mfs.File:
		return nil
	case *mfs.Directory:
		names, err := fsn.ListNames(adder.ctx)
		if err != nil {
			return err
		}

		for _, name := range names {
			child, err := fsn.Child(name)
			if err != nil {
				return err
			}

			childpath := gopath.Join(path, name)
			err = adder.outputDirs(childpath, child)
			if err != nil {
				return err
			}

			fsn.Uncache(name)
		}
		nd, err := fsn.GetNode()
		if err != nil {
			return err
		}

		return outputDagnode(adder.Out, path, nd)
	default:
		return fmt.Errorf("unrecognized fsn type: %#v", fsn)
	}
}

// Add builds a merkledag node from a reader, adds it to the blockstore,
// and returns the key representing that node.
// If you want to pin it, use NewAdder() and Adder.PinRoot().
func Add(n *core.IpfsNode, r io.Reader) (string, error) {
	return AddWithContext(n.Context(), n, r)
}

// AddWithContext does the same as Add, but with a custom context.
func AddWithContext(ctx context.Context, n *core.IpfsNode, r io.Reader) (string, error) {
	defer n.Blockstore.PinLock().Unlock()

	fileAdder, err := NewAdder(ctx, n.Pinning, n.Blockstore, n.DAG)
	if err != nil {
		return "", err
	}

	node, err := fileAdder.add(r)
	if err != nil {
		return "", err
	}

	return node.Cid().String(), nil
}

// AddR recursively adds files in |path|.
func AddR(n *core.IpfsNode, root string) (key string, err error) {
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

	fileAdder, err := NewAdder(n.Context(), n.Pinning, n.Blockstore, n.DAG)
	if err != nil {
		return "", err
	}

	err = fileAdder.addFile(f)
	if err != nil {
		return "", err
	}

	nd, err := fileAdder.Finalize()
	if err != nil {
		return "", err
	}

	return nd.String(), nil
}

// AddWrapped adds data from a reader, and wraps it with a directory object
// to preserve the filename.
// Returns the path of the added file ("<dir hash>/filename"), the DAG node of
// the directory, and and error if any.
func AddWrapped(n *core.IpfsNode, r io.Reader, filename string) (string, ipld.Node, error) {
	file := files.NewReaderFile(filename, filename, ioutil.NopCloser(r), nil)
	fileAdder, err := NewAdder(n.Context(), n.Pinning, n.Blockstore, n.DAG)
	if err != nil {
		return "", nil, err
	}
	fileAdder.Wrap = true

	defer n.Blockstore.PinLock().Unlock()

	err = fileAdder.addFile(file)
	if err != nil {
		return "", nil, err
	}

	dagnode, err := fileAdder.Finalize()
	if err != nil {
		return "", nil, err
	}

	c := dagnode.Cid()
	return gopath.Join(c.String(), filename), dagnode, nil
}

func (adder *Adder) addNode(node ipld.Node, path string) error {
	// patch it into the root
	if path == "" {
		path = node.Cid().String()
	}

	if pi, ok := node.(*posinfo.FilestoreNode); ok {
		node = pi.Node
	}

	mr, err := adder.mfsRoot()
	if err != nil {
		return err
	}
	dir := gopath.Dir(path)
	if dir != "." {
		opts := mfs.MkdirOpts{
			Mkparents:  true,
			Flush:      false,
			CidBuilder: adder.CidBuilder,
		}
		if err := mfs.Mkdir(mr, dir, opts); err != nil {
			return err
		}
	}

	if err := mfs.PutNode(mr, path, node); err != nil {
		return err
	}

	if !adder.Silent {
		return outputDagnode(adder.Out, path, node)
	}
	return nil
}

// AddAllAndPin adds the given request's files and pin them.
func (adder *Adder) AddAllAndPin(file files.File) (ipld.Node, error) {
	if adder.Pin {
		adder.unlocker = adder.blockstore.PinLock()
	}
	defer func() {
		if adder.unlocker != nil {
			adder.unlocker.Unlock()
		}
	}()

	switch {
	case file.IsDirectory():
		// Iterate over each top-level file and add individually. Otherwise the
		// single files.File f is treated as a directory, affecting hidden file
		// semantics.
		for {
			f, err := file.NextFile()
			if err == io.EOF {
				// Finished the list of files.
				break
			} else if err != nil {
				return nil, err
			}
			if err := adder.addFile(f); err != nil {
				return nil, err
			}
		}
		break
	default:
		if err := adder.addFile(file); err != nil {
			return nil, err
		}
		break
	}

	// copy intermediary nodes from editor to our actual dagservice
	nd, err := adder.Finalize()
	if err != nil {
		return nil, err
	}

	if !adder.Pin {
		return nd, nil
	}
	return nd, adder.PinRoot()
}

func (adder *Adder) addFile(file files.File) error {
	err := adder.maybePauseForGC()
	if err != nil {
		return err
	}

	if adder.liveNodes >= liveCacheSize {
		// TODO: A smarter cache that uses some sort of lru cache with an eviction handler
		mr, err := adder.mfsRoot()
		if err != nil {
			return err
		}
		if err := mr.FlushMemFree(adder.ctx); err != nil {
			return err
		}

		adder.liveNodes = 0
	}
	adder.liveNodes++

	if file.IsDirectory() {
		return adder.addDir(file)
	}

	// case for symlink
	if s, ok := file.(*files.Symlink); ok {
		sdata, err := unixfs.SymlinkData(s.Target)
		if err != nil {
			return err
		}

		dagnode := dag.NodeWithData(sdata)
		dagnode.SetCidBuilder(adder.CidBuilder)
		err = adder.dagService.Add(adder.ctx, dagnode)
		if err != nil {
			return err
		}

		return adder.addNode(dagnode, s.FileName())
	}

	// case for regular file
	// if the progress flag was specified, wrap the file so that we can send
	// progress updates to the client (over the output channel)
	var reader io.Reader = file
	if adder.Progress {
		rdr := &progressReader{file: file, out: adder.Out}
		if fi, ok := file.(files.FileInfo); ok {
			reader = &progressReader2{rdr, fi}
		} else {
			reader = rdr
		}
	}

	dagnode, err := adder.add(reader)
	if err != nil {
		return err
	}

	addFileName := file.FileName()
	addFileInfo, ok := file.(files.FileInfo)
	if ok {
		if addFileInfo.AbsPath() == os.Stdin.Name() && adder.Name != "" {
			addFileName = adder.Name
			adder.Name = ""
		}
	}
	// patch it into the root
	return adder.addNode(dagnode, addFileName)
}

func (adder *Adder) addDir(dir files.File) error {
	log.Infof("adding directory: %s", dir.FileName())

	mr, err := adder.mfsRoot()
	if err != nil {
		return err
	}
	err = mfs.Mkdir(mr, dir.FileName(), mfs.MkdirOpts{
		Mkparents:  true,
		Flush:      false,
		CidBuilder: adder.CidBuilder,
	})
	if err != nil {
		return err
	}

	for {
		file, err := dir.NextFile()
		if err != nil && err != io.EOF {
			return err
		}
		if file == nil {
			break
		}

		// Skip hidden files when adding recursively, unless Hidden is enabled.
		if files.IsHidden(file) && !adder.Hidden {
			log.Infof("%s is hidden, skipping", file.FileName())
			continue
		}
		err = adder.addFile(file)
		if err != nil {
			return err
		}
	}

	return nil
}

func (adder *Adder) maybePauseForGC() error {
	if adder.unlocker != nil && adder.blockstore.GCRequested() {
		err := adder.PinRoot()
		if err != nil {
			return err
		}

		adder.unlocker.Unlock()
		adder.unlocker = adder.blockstore.PinLock()
	}
	return nil
}

// outputDagnode sends dagnode info over the output channel
func outputDagnode(out chan<- interface{}, name string, dn ipld.Node) error {
	if out == nil {
		return nil
	}

	o, err := getOutput(dn)
	if err != nil {
		return err
	}

	out <- &coreiface.AddEvent{
		Hash: o.Hash,
		Name: name,
		Size: o.Size,
	}

	return nil
}

// from core/commands/object.go
func getOutput(dagnode ipld.Node) (*Object, error) {
	c := dagnode.Cid()
	s, err := dagnode.Size()
	if err != nil {
		return nil, err
	}

	output := &Object{
		Hash:  c.String(),
		Size:  strconv.FormatUint(s, 10),
		Links: make([]Link, len(dagnode.Links())),
	}

	for i, link := range dagnode.Links() {
		output.Links[i] = Link{
			Name: link.Name,
			Size: link.Size,
		}
	}

	return output, nil
}

type progressReader struct {
	file         files.File
	out          chan<- interface{}
	bytes        int64
	lastProgress int64
}

func (i *progressReader) Read(p []byte) (int, error) {
	n, err := i.file.Read(p)

	i.bytes += int64(n)
	if i.bytes-i.lastProgress >= progressReaderIncrement || err == io.EOF {
		i.lastProgress = i.bytes
		i.out <- &coreiface.AddEvent{
			Name:  i.file.FileName(),
			Bytes: i.bytes,
		}
	}

	return n, err
}

type progressReader2 struct {
	*progressReader
	files.FileInfo
}
