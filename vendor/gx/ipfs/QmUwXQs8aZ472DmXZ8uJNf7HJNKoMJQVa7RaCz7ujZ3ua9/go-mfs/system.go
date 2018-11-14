// package mfs implements an in memory model of a mutable IPFS filesystem.
//
// It consists of four main structs:
// 1) The Filesystem
//        The filesystem serves as a container and entry point for various mfs filesystems
// 2) Root
//        Root represents an individual filesystem mounted within the mfs system as a whole
// 3) Directories
// 4) Files
package mfs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	dag "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag"
	ft "gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
)

var ErrNotExist = errors.New("no such rootfs")

var log = logging.Logger("mfs")

var ErrIsDirectory = errors.New("error: is a directory")

type childCloser interface {
	closeChild(string, ipld.Node, bool) error
}

type NodeType int

const (
	TFile NodeType = iota
	TDir
)

// FSNode represents any node (directory, root, or file) in the mfs filesystem.
type FSNode interface {
	GetNode() (ipld.Node, error)
	Flush() error
	Type() NodeType
}

// IsDir checks whether the FSNode is dir type
func IsDir(fsn FSNode) bool {
	return fsn.Type() == TDir
}

// IsFile checks whether the FSNode is file type
func IsFile(fsn FSNode) bool {
	return fsn.Type() == TFile
}

// Root represents the root of a filesystem tree.
type Root struct {

	// Root directory of the MFS layout.
	dir *Directory

	repub *Republisher
}

// PubFunc is the function used by the `publish()` method.
type PubFunc func(context.Context, cid.Cid) error

// NewRoot creates a new Root and starts up a republisher routine for it.
func NewRoot(parent context.Context, ds ipld.DAGService, node *dag.ProtoNode, pf PubFunc) (*Root, error) {

	var repub *Republisher
	if pf != nil {
		repub = NewRepublisher(parent, pf, time.Millisecond*300, time.Second*3)
		repub.setVal(node.Cid())
		go repub.Run()
	}

	root := &Root{
		repub: repub,
	}

	fsn, err := ft.FSNodeFromBytes(node.Data())
	if err != nil {
		log.Error("IPNS pointer was not unixfs node")
		return nil, err
	}

	switch fsn.Type() {
	case ft.TDirectory, ft.THAMTShard:
		newDir, err := NewDirectory(parent, node.String(), node, root, ds)
		if err != nil {
			return nil, err
		}

		root.dir = newDir
	case ft.TFile, ft.TMetadata, ft.TRaw:
		return nil, fmt.Errorf("root can't be a file (unixfs type: %s)", fsn.Type())
	default:
		return nil, fmt.Errorf("unrecognized unixfs type: %s", fsn.Type())
	}
	return root, nil
}

// GetDirectory returns the root directory.
func (kr *Root) GetDirectory() *Directory {
	return kr.dir
}

// Flush signals that an update has occurred since the last publish,
// and updates the Root republisher.
func (kr *Root) Flush() error {
	nd, err := kr.GetDirectory().GetNode()
	if err != nil {
		return err
	}

	if kr.repub != nil {
		kr.repub.Update(nd.Cid())
	}
	return nil
}

// FlushMemFree flushes the root directory and then uncaches all of its links.
// This has the effect of clearing out potentially stale references and allows
// them to be garbage collected.
// CAUTION: Take care not to ever call this while holding a reference to any
// child directories. Those directories will be bad references and using them
// may have unintended racy side effects.
// A better implemented mfs system (one that does smarter internal caching and
// refcounting) shouldnt need this method.
func (kr *Root) FlushMemFree(ctx context.Context) error {
	dir := kr.GetDirectory()

	if err := dir.Flush(); err != nil {
		return err
	}

	dir.lock.Lock()
	defer dir.lock.Unlock()
	for name := range dir.files {
		delete(dir.files, name)
	}
	for name := range dir.childDirs {
		delete(dir.childDirs, name)
	}

	return nil
}

// closeChild implements the childCloser interface, and signals to the publisher that
// there are changes ready to be published.
func (kr *Root) closeChild(name string, nd ipld.Node, sync bool) error {
	err := kr.GetDirectory().dserv.Add(context.TODO(), nd)
	if err != nil {
		return err
	}

	if kr.repub != nil {
		kr.repub.Update(nd.Cid())
	}
	return nil
}

func (kr *Root) Close() error {
	nd, err := kr.GetDirectory().GetNode()
	if err != nil {
		return err
	}

	if kr.repub != nil {
		kr.repub.Update(nd.Cid())
		return kr.repub.Close()
	}

	return nil
}

// Republisher manages when to publish a given entry.
type Republisher struct {
	TimeoutLong  time.Duration
	TimeoutShort time.Duration
	Publish      chan struct{}
	pubfunc      PubFunc
	pubnowch     chan chan struct{}

	ctx    context.Context
	cancel func()

	lk      sync.Mutex
	val     cid.Cid
	lastpub cid.Cid
}

// NewRepublisher creates a new Republisher object to republish the given root
// using the given short and long time intervals.
func NewRepublisher(ctx context.Context, pf PubFunc, tshort, tlong time.Duration) *Republisher {
	ctx, cancel := context.WithCancel(ctx)
	return &Republisher{
		TimeoutShort: tshort,
		TimeoutLong:  tlong,
		Publish:      make(chan struct{}, 1),
		pubfunc:      pf,
		pubnowch:     make(chan chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (p *Republisher) setVal(c cid.Cid) {
	p.lk.Lock()
	defer p.lk.Unlock()
	p.val = c
}

// WaitPub Returns immediately if `lastpub` value is consistent with the
// current value `val`, else will block until `val` has been published.
func (p *Republisher) WaitPub() {
	p.lk.Lock()
	consistent := p.lastpub == p.val
	p.lk.Unlock()
	if consistent {
		return
	}

	wait := make(chan struct{})
	p.pubnowch <- wait
	<-wait
}

func (p *Republisher) Close() error {
	err := p.publish(p.ctx)
	p.cancel()
	return err
}

// Touch signals that an update has occurred since the last publish.
// Multiple consecutive touches may extend the time period before
// the next Publish occurs in order to more efficiently batch updates.
func (np *Republisher) Update(c cid.Cid) {
	np.setVal(c)
	select {
	case np.Publish <- struct{}{}:
	default:
	}
}

// Run is the main republisher loop.
func (np *Republisher) Run() {
	for {
		select {
		case <-np.Publish:
			quick := time.After(np.TimeoutShort)
			longer := time.After(np.TimeoutLong)

		wait:
			var pubnowresp chan struct{}

			select {
			case <-np.ctx.Done():
				return
			case <-np.Publish:
				quick = time.After(np.TimeoutShort)
				goto wait
			case <-quick:
			case <-longer:
			case pubnowresp = <-np.pubnowch:
			}

			err := np.publish(np.ctx)
			if pubnowresp != nil {
				pubnowresp <- struct{}{}
			}
			if err != nil {
				log.Errorf("republishRoot error: %s", err)
			}

		case <-np.ctx.Done():
			return
		}
	}
}

// publish calls the `PubFunc`.
func (np *Republisher) publish(ctx context.Context) error {
	np.lk.Lock()
	topub := np.val
	np.lk.Unlock()

	err := np.pubfunc(ctx, topub)
	if err != nil {
		return err
	}
	np.lk.Lock()
	np.lastpub = topub
	np.lk.Unlock()
	return nil
}
