package amt

import (
	"bytes"
	"context"
	"fmt"
	"math/bits"

	cid "github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	cbg "github.com/whyrusleeping/cbor-gen"
)

var log = logging.Logger("amt")

const (
	// Width must be a power of 2. We set this to 8.
	maxIndexBits = 63
	widthBits    = 3
	width        = 1 << widthBits             // 8
	bitfieldSize = 1                          // ((width - 1) >> 3) + 1
	maxHeight    = maxIndexBits/widthBits - 1 // 20 (because the root is at height 0).
)

// MaxIndex is the maximum index for elements in the AMT. This is currently 1^63
// (max int) because the width is 8. That means every "level" consumes 3 bits
// from the index, and 63/3 is a nice even 21
const MaxIndex = uint64(1<<maxIndexBits) - 1

type Root struct {
	Height uint64
	Count  uint64
	Node   Node

	store cbor.IpldStore
}

type Node struct {
	Bmap   [bitfieldSize]byte
	Links  []cid.Cid
	Values []*cbg.Deferred

	expLinks []cid.Cid
	expVals  []*cbg.Deferred
	cache    []*Node
}

func NewAMT(bs cbor.IpldStore) *Root {
	return &Root{
		store: bs,
	}
}

func LoadAMT(ctx context.Context, bs cbor.IpldStore, c cid.Cid) (*Root, error) {
	var r Root
	if err := bs.Get(ctx, c, &r); err != nil {
		return nil, err
	}
	if r.Height > maxHeight {
		return nil, fmt.Errorf("failed to load AMT: height out of bounds: %d > %d", r.Height, maxHeight)
	}

	r.store = bs

	return &r, nil
}

func (r *Root) Set(ctx context.Context, i uint64, val interface{}) error {
	if i > MaxIndex {
		return fmt.Errorf("index %d is out of range for the amt", i)
	}

	var b []byte
	if m, ok := val.(cbg.CBORMarshaler); ok {
		buf := new(bytes.Buffer)
		if err := m.MarshalCBOR(buf); err != nil {
			return err
		}
		b = buf.Bytes()
	} else {
		var err error
		b, err = cbor.DumpObject(val)
		if err != nil {
			return err
		}
	}

	for i >= nodesForHeight(int(r.Height)+1) {
		if !r.Node.empty() {
			if err := r.Node.Flush(ctx, r.store, int(r.Height)); err != nil {
				return err
			}

			c, err := r.store.Put(ctx, &r.Node)
			if err != nil {
				return err
			}

			r.Node = Node{
				Bmap:  [...]byte{0x01},
				Links: []cid.Cid{c},
			}
		}
		r.Height++
	}

	addVal, err := r.Node.set(ctx, r.store, int(r.Height), i, &cbg.Deferred{Raw: b})
	if err != nil {
		return err
	}

	if addVal {
		r.Count++
	}

	return nil
}

func FromArray(ctx context.Context, bs cbor.IpldStore, vals []cbg.CBORMarshaler) (cid.Cid, error) {
	r := NewAMT(bs)
	if err := r.BatchSet(ctx, vals); err != nil {
		return cid.Undef, err
	}

	return r.Flush(ctx)
}

func (r *Root) BatchSet(ctx context.Context, vals []cbg.CBORMarshaler) error {
	// TODO: there are more optimized ways of doing this method
	for i, v := range vals {
		if err := r.Set(ctx, uint64(i), v); err != nil {
			return err
		}
	}
	return nil
}

func (r *Root) Get(ctx context.Context, i uint64, out interface{}) error {
	if i > MaxIndex {
		return fmt.Errorf("index %d is out of range for the amt", i)
	}

	if i >= nodesForHeight(int(r.Height+1)) {
		return &ErrNotFound{Index: i}
	}
	return r.Node.get(ctx, r.store, int(r.Height), i, out)
}

func (n *Node) get(ctx context.Context, bs cbor.IpldStore, height int, i uint64, out interface{}) error {
	subi := i / nodesForHeight(height)
	set, _ := n.getBit(subi)
	if !set {
		return &ErrNotFound{i}
	}
	if height == 0 {
		n.expandValues()

		d := n.expVals[i]

		if um, ok := out.(cbg.CBORUnmarshaler); ok {
			return um.UnmarshalCBOR(bytes.NewReader(d.Raw))
		} else {
			return cbor.DecodeInto(d.Raw, out)
		}
	}

	subn, err := n.loadNode(ctx, bs, subi, false)
	if err != nil {
		return err
	}

	return subn.get(ctx, bs, height-1, i%nodesForHeight(height), out)
}

func (r *Root) BatchDelete(ctx context.Context, indices []uint64) error {
	// TODO: theres a faster way of doing this, but this works for now
	for _, i := range indices {
		if err := r.Delete(ctx, i); err != nil {
			return err
		}
	}

	return nil
}

func (r *Root) Delete(ctx context.Context, i uint64) error {
	if i > MaxIndex {
		return fmt.Errorf("index %d is out of range for the amt", i)
	}
	//fmt.Printf("i: %d, h: %d, nfh: %d\n", i, r.Height, nodesForHeight(int(r.Height)))
	if i >= nodesForHeight(int(r.Height+1)) {
		return &ErrNotFound{i}
	}

	if err := r.Node.delete(ctx, r.store, int(r.Height), i); err != nil {
		return err
	}
	r.Count--

	for r.Node.Bmap[0] == 1 && r.Height > 0 {
		sub, err := r.Node.loadNode(ctx, r.store, 0, false)
		if err != nil {
			return err
		}

		r.Node = *sub
		r.Height--
	}

	return nil
}

func (n *Node) delete(ctx context.Context, bs cbor.IpldStore, height int, i uint64) error {
	subi := i / nodesForHeight(height)
	set, _ := n.getBit(subi)
	if !set {
		return &ErrNotFound{i}
	}
	if height == 0 {
		n.expandValues()

		n.expVals[i] = nil
		n.clearBit(i)

		return nil
	}

	subn, err := n.loadNode(ctx, bs, subi, false)
	if err != nil {
		return err
	}

	if err := subn.delete(ctx, bs, height-1, i%nodesForHeight(height)); err != nil {
		return err
	}

	if subn.empty() {
		n.clearBit(subi)
		n.cache[subi] = nil
		n.expLinks[subi] = cid.Undef
	}

	return nil
}

// Subtract removes all elements of 'or' from 'r'
func (r *Root) Subtract(ctx context.Context, or *Root) error {
	// TODO: as with other methods, there should be an optimized way of doing this
	return or.ForEach(ctx, func(i uint64, _ *cbg.Deferred) error {
		return r.Delete(ctx, i)
	})
}

func (r *Root) ForEach(ctx context.Context, cb func(uint64, *cbg.Deferred) error) error {
	return r.Node.forEachAt(ctx, r.store, int(r.Height), 0, 0, cb)
}

func (r *Root) ForEachAt(ctx context.Context, start uint64, cb func(uint64, *cbg.Deferred) error) error {
	return r.Node.forEachAt(ctx, r.store, int(r.Height), start, 0, cb)
}

func (n *Node) forEachAt(ctx context.Context, bs cbor.IpldStore, height int, start, offset uint64, cb func(uint64, *cbg.Deferred) error) error {
	if height == 0 {
		n.expandValues()

		for i, v := range n.expVals {
			if v != nil {
				ix := offset + uint64(i)
				if ix < start {
					continue
				}

				if err := cb(offset+uint64(i), v); err != nil {
					return err
				}
			}
		}

		return nil
	}

	if n.cache == nil {
		n.expandLinks()
	}

	subCount := nodesForHeight(height)
	for i, v := range n.expLinks {
		var sub Node
		if n.cache[i] != nil {
			sub = *n.cache[i]
		} else if v != cid.Undef {
			if err := bs.Get(ctx, v, &sub); err != nil {
				return err
			}
		} else {
			continue
		}

		offs := offset + (uint64(i) * subCount)
		nextOffs := offs + subCount
		if start >= nextOffs {
			continue
		}

		if err := sub.forEachAt(ctx, bs, height-1, start, offs, cb); err != nil {
			return err
		}
	}
	return nil

}

func (r *Root) FirstSetIndex(ctx context.Context) (uint64, error) {
	return r.Node.firstSetIndex(ctx, r.store, int(r.Height))
}

var errNoVals = fmt.Errorf("no values")

func (n *Node) firstSetIndex(ctx context.Context, bs cbor.IpldStore, height int) (uint64, error) {
	if height == 0 {
		n.expandValues()
		for i, v := range n.expVals {
			if v != nil {
				return uint64(i), nil
			}
		}
		// Would be really weird if we ever actually hit this
		return 0, errNoVals
	}

	if n.cache == nil {
		n.expandLinks()
	}

	for i := 0; i < width; i++ {
		ok, _ := n.getBit(uint64(i))
		if ok {
			subn, err := n.loadNode(ctx, bs, uint64(i), false)
			if err != nil {
				return 0, err
			}

			ix, err := subn.firstSetIndex(ctx, bs, height-1)
			if err != nil {
				return 0, err
			}

			subCount := nodesForHeight(height)
			return ix + (uint64(i) * subCount), nil
		}
	}

	return 0, errNoVals
}

func (n *Node) expandValues() {
	if len(n.expVals) == 0 {
		n.expVals = make([]*cbg.Deferred, width)
		for x := uint64(0); x < width; x++ {
			set, ix := n.getBit(x)
			if set {
				n.expVals[x] = n.Values[ix]
			}
		}
	}
}

func (n *Node) set(ctx context.Context, bs cbor.IpldStore, height int, i uint64, val *cbg.Deferred) (bool, error) {
	//nfh := nodesForHeight(height)
	//fmt.Printf("[set] h: %d, i: %d, subi: %d\n", height, i, i/nfh)
	if height == 0 {
		n.expandValues()
		alreadySet, _ := n.getBit(i)
		n.expVals[i] = val
		n.setBit(i)

		return !alreadySet, nil
	}

	nfh := nodesForHeight(height)

	subn, err := n.loadNode(ctx, bs, i/nfh, true)
	if err != nil {
		return false, err
	}

	return subn.set(ctx, bs, height-1, i%nfh, val)
}

func (n *Node) getBit(i uint64) (bool, int) {
	if i > 7 {
		panic("cant deal with wider arrays yet")
	}

	if len(n.Bmap) == 0 {
		return false, 0
	}

	if n.Bmap[0]&byte(1<<i) == 0 {
		return false, 0
	}

	mask := byte((1 << i) - 1)
	return true, bits.OnesCount8(n.Bmap[0] & mask)
}

func (n *Node) setBit(i uint64) {
	if i > 7 {
		panic("cant deal with wider arrays yet")
	}

	if len(n.Bmap) == 0 {
		n.Bmap = [...]byte{0}
	}

	n.Bmap[0] = n.Bmap[0] | byte(1<<i)
}

func (n *Node) clearBit(i uint64) {
	if i > 7 {
		panic("cant deal with wider arrays yet")
	}

	if len(n.Bmap) == 0 {
		panic("invariant violated: called clear bit on empty node")
	}

	mask := byte(0xff - (1 << i))

	n.Bmap[0] = n.Bmap[0] & mask
}

func (n *Node) expandLinks() {
	n.cache = make([]*Node, width)
	n.expLinks = make([]cid.Cid, width)
	for x := uint64(0); x < width; x++ {
		set, ix := n.getBit(x)
		if set {
			n.expLinks[x] = n.Links[ix]
		}
	}
}

func (n *Node) loadNode(ctx context.Context, bs cbor.IpldStore, i uint64, create bool) (*Node, error) {
	if n.cache == nil {
		n.expandLinks()
	} else {
		if n := n.cache[i]; n != nil {
			return n, nil
		}
	}

	set, _ := n.getBit(i)

	var subn *Node
	if set {
		var sn Node
		if err := bs.Get(ctx, n.expLinks[i], &sn); err != nil {
			return nil, err
		}

		subn = &sn
	} else {
		if create {
			subn = &Node{}
			n.setBit(i)
		} else {
			return nil, fmt.Errorf("no node found at (sub)index %d", i)
		}
	}
	n.cache[i] = subn

	return subn, nil
}

func nodesForHeight(height int) uint64 {
	heightLogTwo := uint64(widthBits * height)
	if heightLogTwo >= 64 {
		// Should never happen. Max height is checked at all entry points.
		panic("height overflow")
	}
	return 1 << heightLogTwo
}

func (r *Root) Flush(ctx context.Context) (cid.Cid, error) {
	if err := r.Node.Flush(ctx, r.store, int(r.Height)); err != nil {
		return cid.Undef, err
	}

	return r.store.Put(ctx, r)
}

func (n *Node) empty() bool {
	return len(n.Bmap) == 0 || n.Bmap[0] == 0
}

func (n *Node) Flush(ctx context.Context, bs cbor.IpldStore, depth int) error {
	if depth == 0 {
		if len(n.expVals) == 0 {
			return nil
		}
		n.Values = nil
		for i := uint64(0); i < width; i++ {
			v := n.expVals[i]
			if v != nil {
				n.Values = append(n.Values, v)
				n.setBit(i)
			}
		}
		return nil
	}

	if len(n.expLinks) == 0 {
		// nothing to do!
		return nil
	}

	n.Bmap = [...]byte{0}
	n.Links = nil

	for i := uint64(0); i < width; i++ {
		subn := n.cache[i]
		if subn != nil {
			if err := subn.Flush(ctx, bs, depth-1); err != nil {
				return err
			}

			c, err := bs.Put(ctx, subn)
			if err != nil {
				return err
			}
			n.expLinks[i] = c
		}

		l := n.expLinks[i]
		if l != cid.Undef {
			n.Links = append(n.Links, l)
			n.setBit(i)
		}
	}

	return nil
}

type ErrNotFound struct {
	Index uint64
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("Index %d not found in AMT", e.Index)
}

func (e ErrNotFound) NotFound() bool {
	return true
}
