package merkledag

import (
	"fmt"
	"sort"
	"strings"

	"gx/ipfs/QmRcHuYzAyswytBuMF78rj3LTChYszomRFXNg4685ZN1WM/go-block-format"

	pb "gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag/pb"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
)

// Make sure the user doesn't upgrade this file.
// We need to check *here* as well as inside the `pb` package *just* in case the
// user replaces *all* go files in that package.
const _ = pb.DoNotUpgradeFileEverItWillChangeYourHashes

// for now, we use a PBNode intermediate thing.
// because native go objects are nice.

// unmarshal decodes raw data into a *Node instance.
// The conversion uses an intermediate PBNode.
func (n *ProtoNode) unmarshal(encoded []byte) error {
	var pbn pb.PBNode
	if err := pbn.Unmarshal(encoded); err != nil {
		return fmt.Errorf("unmarshal failed. %v", err)
	}

	pbnl := pbn.GetLinks()
	n.links = make([]*ipld.Link, len(pbnl))
	for i, l := range pbnl {
		n.links[i] = &ipld.Link{Name: l.GetName(), Size: l.GetTsize()}
		c, err := cid.Cast(l.GetHash())
		if err != nil {
			return fmt.Errorf("link hash #%d is not valid multihash. %v", i, err)
		}
		n.links[i].Cid = c
	}
	sort.Stable(LinkSlice(n.links)) // keep links sorted

	n.data = pbn.GetData()
	n.encoded = encoded
	return nil
}

// Marshal encodes a *Node instance into a new byte slice.
// The conversion uses an intermediate PBNode.
func (n *ProtoNode) Marshal() ([]byte, error) {
	pbn := n.getPBNode()
	data, err := pbn.Marshal()
	if err != nil {
		return data, fmt.Errorf("marshal failed. %v", err)
	}
	return data, nil
}

func (n *ProtoNode) getPBNode() *pb.PBNode {
	pbn := &pb.PBNode{}
	if len(n.links) > 0 {
		pbn.Links = make([]*pb.PBLink, len(n.links))
	}

	sort.Stable(LinkSlice(n.links)) // keep links sorted
	for i, l := range n.links {
		pbn.Links[i] = &pb.PBLink{}
		pbn.Links[i].Name = &l.Name
		pbn.Links[i].Tsize = &l.Size
		if l.Cid.Defined() {
			pbn.Links[i].Hash = l.Cid.Bytes()
		}
	}

	if len(n.data) > 0 {
		pbn.Data = n.data
	}
	return pbn
}

// EncodeProtobuf returns the encoded raw data version of a Node instance.
// It may use a cached encoded version, unless the force flag is given.
func (n *ProtoNode) EncodeProtobuf(force bool) ([]byte, error) {
	sort.Stable(LinkSlice(n.links)) // keep links sorted
	if n.encoded == nil || force {
		n.cached = cid.Undef
		var err error
		n.encoded, err = n.Marshal()
		if err != nil {
			return nil, err
		}
	}

	if !n.cached.Defined() {
		c, err := n.CidBuilder().Sum(n.encoded)
		if err != nil {
			return nil, err
		}

		n.cached = c
	}

	return n.encoded, nil
}

// DecodeProtobuf decodes raw data and returns a new Node instance.
func DecodeProtobuf(encoded []byte) (*ProtoNode, error) {
	n := new(ProtoNode)
	err := n.unmarshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("incorrectly formatted merkledag node: %s", err)
	}
	return n, nil
}

// DecodeProtobufBlock is a block decoder for protobuf IPLD nodes conforming to
// node.DecodeBlockFunc
func DecodeProtobufBlock(b blocks.Block) (ipld.Node, error) {
	c := b.Cid()
	if c.Type() != cid.DagProtobuf {
		return nil, fmt.Errorf("this function can only decode protobuf nodes")
	}

	decnd, err := DecodeProtobuf(b.RawData())
	if err != nil {
		if strings.Contains(err.Error(), "Unmarshal failed") {
			return nil, fmt.Errorf("the block referred to by '%s' was not a valid merkledag node", c)
		}
		return nil, fmt.Errorf("failed to decode Protocol Buffers: %v", err)
	}

	decnd.cached = c
	decnd.SetCidBuilder(c.Prefix())
	return decnd, nil
}

// Type assertion
var _ ipld.DecodeBlockFunc = DecodeProtobufBlock
