package io

import (
	"context"
	"errors"
	"io"

	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
	mdag "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag"
	ft "gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs"
)

// Common errors
var (
	ErrIsDir            = errors.New("this dag node is a directory")
	ErrCantReadSymlinks = errors.New("cannot currently read symlinks")
	ErrUnkownNodeType   = errors.New("unknown node type")
)

// A DagReader provides read-only read and seek acess to a unixfs file.
// Different implementations of readers are used for the different
// types of unixfs/protobuf-encoded nodes.
type DagReader interface {
	ReadSeekCloser
	Size() uint64
	CtxReadFull(context.Context, []byte) (int, error)
}

// A ReadSeekCloser implements interfaces to read, copy, seek and close.
type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
	io.WriterTo
}

// NewDagReader creates a new reader object that reads the data represented by
// the given node, using the passed in DAGService for data retrieval
func NewDagReader(ctx context.Context, n ipld.Node, serv ipld.NodeGetter) (DagReader, error) {
	switch n := n.(type) {
	case *mdag.RawNode:
		return NewBufDagReader(n.RawData()), nil
	case *mdag.ProtoNode:
		fsNode, err := ft.FSNodeFromBytes(n.Data())
		if err != nil {
			return nil, err
		}

		switch fsNode.Type() {
		case ft.TDirectory, ft.THAMTShard:
			// Dont allow reading directories
			return nil, ErrIsDir
		case ft.TFile, ft.TRaw:
			return NewPBFileReader(ctx, n, fsNode, serv), nil
		case ft.TMetadata:
			if len(n.Links()) == 0 {
				return nil, errors.New("incorrectly formatted metadata object")
			}
			child, err := n.Links()[0].GetNode(ctx, serv)
			if err != nil {
				return nil, err
			}

			childpb, ok := child.(*mdag.ProtoNode)
			if !ok {
				return nil, mdag.ErrNotProtobuf
			}
			return NewDagReader(ctx, childpb, serv)
		case ft.TSymlink:
			return nil, ErrCantReadSymlinks
		default:
			return nil, ft.ErrUnrecognizedType
		}
	default:
		return nil, ErrUnkownNodeType
	}
}
