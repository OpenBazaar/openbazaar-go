package io

import (
	"context"
	"fmt"
	"os"

	mdag "gx/ipfs/QmPJNbVw8o3ohC43ppSXyNXwYKsWShG4zygnirHptfbHri/go-merkledag"

	format "gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs"
	hamt "gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs/hamt"

	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	ipld "gx/ipfs/QmZ6nzCLwGLVfRzYLpD7pW6UNuBDKEcA2imJtVpbEx2rxy/go-ipld-format"
)

// UseHAMTSharding is a global flag that signifies whether or not to use the
// HAMT sharding scheme for directory creation
var UseHAMTSharding = false

// DefaultShardWidth is the default value used for hamt sharding width.
var DefaultShardWidth = 256

// Directory defines a UnixFS directory. It is used for creating, reading and
// editing directories. It allows to work with different directory schemes,
// like the basic or the HAMT implementation.
//
// It just allows to perform explicit edits on a single directory, working with
// directory trees is out of its scope, they are managed by the MFS layer
// (which is the main consumer of this interface).
type Directory interface {

	// SetCidBuilder sets the CID Builder of the root node.
	SetCidBuilder(cid.Builder)

	// AddChild adds a (name, key) pair to the root node.
	AddChild(context.Context, string, ipld.Node) error

	// ForEachLink applies the given function to Links in the directory.
	ForEachLink(context.Context, func(*ipld.Link) error) error

	// EnumLinksAsync returns a channel which will receive Links in the directory
	// as they are enumerated, where order is not gauranteed
	EnumLinksAsync(context.Context) <-chan format.LinkResult

	// Links returns the all the links in the directory node.
	Links(context.Context) ([]*ipld.Link, error)

	// Find returns the root node of the file named 'name' within this directory.
	// In the case of HAMT-directories, it will traverse the tree.
	Find(context.Context, string) (ipld.Node, error)

	// RemoveChild removes the child with the given name.
	RemoveChild(context.Context, string) error

	// GetNode returns the root of this directory.
	GetNode() (ipld.Node, error)

	// GetCidBuilder returns the CID Builder used.
	GetCidBuilder() cid.Builder
}

// TODO: Evaluate removing `dserv` from this layer and providing it in MFS.
// (The functions should in that case add a `DAGService` argument.)

// BasicDirectory is the basic implementation of `Directory`. All the entries
// are stored in a single node.
type BasicDirectory struct {
	node  *mdag.ProtoNode
	dserv ipld.DAGService
}

// HAMTDirectory is the HAMT implementation of `Directory`.
// (See package `hamt` for more information.)
type HAMTDirectory struct {
	shard *hamt.Shard
	dserv ipld.DAGService
}

// NewDirectory returns a Directory. It needs a `DAGService` to add the children.
func NewDirectory(dserv ipld.DAGService) Directory {
	if UseHAMTSharding {
		dir := new(HAMTDirectory)
		s, err := hamt.NewShard(dserv, DefaultShardWidth)
		if err != nil {
			panic(err) // will only panic if DefaultShardWidth is a bad value
		}
		dir.shard = s
		dir.dserv = dserv
		return dir
	}

	dir := new(BasicDirectory)
	dir.node = format.EmptyDirNode()
	dir.dserv = dserv
	return dir
}

// ErrNotADir implies that the given node was not a unixfs directory
var ErrNotADir = fmt.Errorf("merkledag node was not a directory or shard")

// NewDirectoryFromNode loads a unixfs directory from the given IPLD node and
// DAGService.
func NewDirectoryFromNode(dserv ipld.DAGService, node ipld.Node) (Directory, error) {
	protoBufNode, ok := node.(*mdag.ProtoNode)
	if !ok {
		return nil, ErrNotADir
	}

	fsNode, err := format.FSNodeFromBytes(protoBufNode.Data())
	if err != nil {
		return nil, err
	}

	switch fsNode.Type() {
	case format.TDirectory:
		return &BasicDirectory{
			dserv: dserv,
			node:  protoBufNode.Copy().(*mdag.ProtoNode),
		}, nil
	case format.THAMTShard:
		shard, err := hamt.NewHamtFromDag(dserv, node)
		if err != nil {
			return nil, err
		}
		return &HAMTDirectory{
			dserv: dserv,
			shard: shard,
		}, nil
	}

	return nil, ErrNotADir
}

// SetCidBuilder implements the `Directory` interface.
func (d *BasicDirectory) SetCidBuilder(builder cid.Builder) {
	d.node.SetCidBuilder(builder)
}

// AddChild implements the `Directory` interface. It adds (or replaces)
// a link to the given `node` under `name`.
func (d *BasicDirectory) AddChild(ctx context.Context, name string, node ipld.Node) error {
	d.node.RemoveNodeLink(name)
	// Remove old link (if it existed), don't check a potential `ErrNotFound`.

	return d.node.AddNodeLink(name, node)
}

// EnumLinksAsync returns a channel which will receive Links in the directory
// as they are enumerated, where order is not gauranteed
func (d *BasicDirectory) EnumLinksAsync(ctx context.Context) <-chan format.LinkResult {
	linkResults := make(chan format.LinkResult)
	go func() {
		defer close(linkResults)
		for _, l := range d.node.Links() {
			select {
			case linkResults <- format.LinkResult{
				Link: l,
				Err:  nil,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return linkResults
}

// ForEachLink implements the `Directory` interface.
func (d *BasicDirectory) ForEachLink(ctx context.Context, f func(*ipld.Link) error) error {
	for _, l := range d.node.Links() {
		if err := f(l); err != nil {
			return err
		}
	}
	return nil
}

// Links implements the `Directory` interface.
func (d *BasicDirectory) Links(ctx context.Context) ([]*ipld.Link, error) {
	return d.node.Links(), nil
}

// Find implements the `Directory` interface.
func (d *BasicDirectory) Find(ctx context.Context, name string) (ipld.Node, error) {
	lnk, err := d.node.GetNodeLink(name)
	if err == mdag.ErrLinkNotFound {
		err = os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}

	return d.dserv.Get(ctx, lnk.Cid)
}

// RemoveChild implements the `Directory` interface.
func (d *BasicDirectory) RemoveChild(ctx context.Context, name string) error {
	return d.node.RemoveNodeLink(name)
}

// GetNode implements the `Directory` interface.
func (d *BasicDirectory) GetNode() (ipld.Node, error) {
	return d.node, nil
}

// GetCidBuilder implements the `Directory` interface.
func (d *BasicDirectory) GetCidBuilder() cid.Builder {
	return d.node.CidBuilder()
}

// SwitchToSharding returns a HAMT implementation of this directory.
func (d *BasicDirectory) SwitchToSharding(ctx context.Context) (Directory, error) {
	hamtDir := new(HAMTDirectory)
	hamtDir.dserv = d.dserv

	shard, err := hamt.NewShard(d.dserv, DefaultShardWidth)
	if err != nil {
		return nil, err
	}
	shard.SetCidBuilder(d.node.CidBuilder())
	hamtDir.shard = shard

	for _, lnk := range d.node.Links() {
		node, err := d.dserv.Get(ctx, lnk.Cid)
		if err != nil {
			return nil, err
		}

		err = hamtDir.shard.Set(ctx, lnk.Name, node)
		if err != nil {
			return nil, err
		}
	}

	return hamtDir, nil
}

// SetCidBuilder implements the `Directory` interface.
func (d *HAMTDirectory) SetCidBuilder(builder cid.Builder) {
	d.shard.SetCidBuilder(builder)
}

// AddChild implements the `Directory` interface.
func (d *HAMTDirectory) AddChild(ctx context.Context, name string, nd ipld.Node) error {
	return d.shard.Set(ctx, name, nd)
}

// ForEachLink implements the `Directory` interface.
func (d *HAMTDirectory) ForEachLink(ctx context.Context, f func(*ipld.Link) error) error {
	return d.shard.ForEachLink(ctx, f)
}

// EnumLinksAsync returns a channel which will receive Links in the directory
// as they are enumerated, where order is not gauranteed
func (d *HAMTDirectory) EnumLinksAsync(ctx context.Context) <-chan format.LinkResult {
	return d.shard.EnumLinksAsync(ctx)
}

// Links implements the `Directory` interface.
func (d *HAMTDirectory) Links(ctx context.Context) ([]*ipld.Link, error) {
	return d.shard.EnumLinks(ctx)
}

// Find implements the `Directory` interface. It will traverse the tree.
func (d *HAMTDirectory) Find(ctx context.Context, name string) (ipld.Node, error) {
	lnk, err := d.shard.Find(ctx, name)
	if err != nil {
		return nil, err
	}

	return lnk.GetNode(ctx, d.dserv)
}

// RemoveChild implements the `Directory` interface.
func (d *HAMTDirectory) RemoveChild(ctx context.Context, name string) error {
	return d.shard.Remove(ctx, name)
}

// GetNode implements the `Directory` interface.
func (d *HAMTDirectory) GetNode() (ipld.Node, error) {
	return d.shard.Node()
}

// GetCidBuilder implements the `Directory` interface.
func (d *HAMTDirectory) GetCidBuilder() cid.Builder {
	return d.shard.CidBuilder()
}
