// Package tar provides functionality to write a unixfs merkledag
// as a tar archive.
package tar

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"path"
	"time"

	mdag "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag"
	ft "gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs"
	uio "gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs/io"

	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
)

// Writer is a utility structure that helps to write
// unixfs merkledag nodes as a tar archive format.
// It wraps any io.Writer.
type Writer struct {
	Dag  ipld.DAGService
	TarW *tar.Writer

	ctx context.Context
}

// NewWriter wraps given io.Writer.
func NewWriter(ctx context.Context, dag ipld.DAGService, w io.Writer) (*Writer, error) {
	return &Writer{
		Dag:  dag,
		TarW: tar.NewWriter(w),
		ctx:  ctx,
	}, nil
}

func (w *Writer) writeDir(nd *mdag.ProtoNode, fpath string) error {
	dir, err := uio.NewDirectoryFromNode(w.Dag, nd)
	if err != nil {
		return err
	}
	if err := writeDirHeader(w.TarW, fpath); err != nil {
		return err
	}

	return dir.ForEachLink(w.ctx, func(l *ipld.Link) error {
		child, err := w.Dag.Get(w.ctx, l.Cid)
		if err != nil {
			return err
		}
		npath := path.Join(fpath, l.Name)
		return w.WriteNode(child, npath)
	})
}

func (w *Writer) writeFile(nd *mdag.ProtoNode, fsNode *ft.FSNode, fpath string) error {
	if err := writeFileHeader(w.TarW, fpath, fsNode.FileSize()); err != nil {
		return err
	}

	dagr := uio.NewPBFileReader(w.ctx, nd, fsNode, w.Dag)
	if _, err := dagr.WriteTo(w.TarW); err != nil {
		return err
	}
	w.TarW.Flush()
	return nil
}

// WriteNode adds a node to the archive.
func (w *Writer) WriteNode(nd ipld.Node, fpath string) error {
	switch nd := nd.(type) {
	case *mdag.ProtoNode:
		fsNode, err := ft.FSNodeFromBytes(nd.Data())
		if err != nil {
			return err
		}

		switch fsNode.Type() {
		case ft.TMetadata:
			fallthrough
		case ft.TDirectory, ft.THAMTShard:
			return w.writeDir(nd, fpath)
		case ft.TRaw:
			fallthrough
		case ft.TFile:
			return w.writeFile(nd, fsNode, fpath)
		case ft.TSymlink:
			return writeSymlinkHeader(w.TarW, string(fsNode.Data()), fpath)
		default:
			return ft.ErrUnrecognizedType
		}
	case *mdag.RawNode:
		if err := writeFileHeader(w.TarW, fpath, uint64(len(nd.RawData()))); err != nil {
			return err
		}

		if _, err := w.TarW.Write(nd.RawData()); err != nil {
			return err
		}
		w.TarW.Flush()
		return nil
	default:
		return fmt.Errorf("nodes of type %T are not supported in unixfs", nd)
	}
}

// Close closes the tar writer.
func (w *Writer) Close() error {
	return w.TarW.Close()
}

func writeDirHeader(w *tar.Writer, fpath string) error {
	return w.WriteHeader(&tar.Header{
		Name:     fpath,
		Typeflag: tar.TypeDir,
		Mode:     0777,
		ModTime:  time.Now(),
		// TODO: set mode, dates, etc. when added to unixFS
	})
}

func writeFileHeader(w *tar.Writer, fpath string, size uint64) error {
	return w.WriteHeader(&tar.Header{
		Name:     fpath,
		Size:     int64(size),
		Typeflag: tar.TypeReg,
		Mode:     0644,
		ModTime:  time.Now(),
		// TODO: set mode, dates, etc. when added to unixFS
	})
}

func writeSymlinkHeader(w *tar.Writer, target, fpath string) error {
	return w.WriteHeader(&tar.Header{
		Name:     fpath,
		Linkname: target,
		Mode:     0777,
		Typeflag: tar.TypeSymlink,
	})
}
