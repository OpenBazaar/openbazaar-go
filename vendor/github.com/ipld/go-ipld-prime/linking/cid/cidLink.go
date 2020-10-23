package cidlink

import (
	"bytes"
	"context"
	"fmt"
	"io"

	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipld/go-ipld-prime"
)

var (
	_ ipld.Link        = Link{}
	_ ipld.LinkBuilder = LinkBuilder{}
)

type Link struct {
	cid.Cid
}

// byteAccessor is a reader interface that can access underlying bytes
type byteAccesor interface {
	Bytes() []byte
}

func (lnk Link) Load(ctx context.Context, lnkCtx ipld.LinkContext, na ipld.NodeAssembler, loader ipld.Loader) error {
	// Open the byte reader.
	r, err := loader(lnk, lnkCtx)
	if err != nil {
		return err
	}
	// Tee into hash checking and unmarshalling.
	mcDecoder, exists := multicodecDecodeTable[lnk.Prefix().Codec]
	if !exists {
		return fmt.Errorf("no decoder registered for multicodec %d", lnk.Prefix().Codec)
	}
	var hasherBytes []byte
	var decodeErr error
	byteBuf, ok := r.(byteAccesor)
	if ok {
		hasherBytes = byteBuf.Bytes()
		decodeErr = mcDecoder(na, r)
	} else {
		var hasher bytes.Buffer // multihash only exports bulk use, which is... really inefficient and should be fixed.
		decodeErr = mcDecoder(na, io.TeeReader(r, &hasher))
		// Error checking order here is tricky.
		//  If decoding errored out, we should still run the reader to the end, to check the hash.
		//  (We still don't implement this by running the hash to the end first, because that would increase the high-water memory requirement.)
		//   ((Which we experience right now anyway because multihash's interface is silly, but we're acting as if that's fixed or will be soon.))
		//  If the hash is rejected, we should return that error (and even if there was a decodeErr, it becomes irrelevant).
		if decodeErr != nil {
			_, err := io.Copy(&hasher, r)
			if err != nil {
				return err
			}
		}
		hasherBytes = hasher.Bytes()
	}

	cid, err := lnk.Prefix().Sum(hasherBytes)
	if err != nil {
		return err
	}
	if cid != lnk.Cid {
		return fmt.Errorf("hash mismatch!  %q (actual) != %q (expected)", cid, lnk.Cid)
	}
	if decodeErr != nil {
		return decodeErr
	}
	return nil
}
func (lnk Link) LinkBuilder() ipld.LinkBuilder {
	return LinkBuilder{lnk.Cid.Prefix()}
}
func (lnk Link) String() string {
	return lnk.Cid.String()
}

type LinkBuilder struct {
	cid.Prefix
}

func (lb LinkBuilder) Build(ctx context.Context, lnkCtx ipld.LinkContext, node ipld.Node, storer ipld.Storer) (ipld.Link, error) {
	// Open the byte writer.
	w, commit, err := storer(lnkCtx)
	if err != nil {
		return nil, err
	}
	// Marshal, teeing into the storage writer and the hasher.
	mcEncoder, exists := multicodecEncodeTable[lb.Prefix.Codec]
	if !exists {
		return nil, fmt.Errorf("no encoder registered for multicodec %d", lb.Prefix.Codec)
	}
	var hasher bytes.Buffer // multihash-via-cid only exports bulk use, which is... really inefficient and should be fixed.
	w = io.MultiWriter(&hasher, w)
	err = mcEncoder(node, w)
	if err != nil {
		return nil, err
	}
	cid, err := lb.Prefix.Sum(hasher.Bytes())
	if err != nil {
		return nil, err
	}
	lnk := Link{cid}
	if err := commit(lnk); err != nil {
		return lnk, err
	}
	return lnk, nil
}
