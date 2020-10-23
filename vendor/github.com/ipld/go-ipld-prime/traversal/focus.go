package traversal

import (
	"fmt"

	ipld "github.com/ipld/go-ipld-prime"
)

// Focus traverses a Node graph according to a path, reaches a single Node,
// and calls the given VisitFn on that reached node.
//
// This function is a helper function which starts a new traversal with default configuration.
// It cannot cross links automatically (since this requires configuration).
// Use the equivalent Focus function on the Progress structure
// for more advanced and configurable walks.
func Focus(n ipld.Node, p ipld.Path, fn VisitFn) error {
	return Progress{}.Focus(n, p, fn)
}

// Get is the equivalent of Focus, but returns the reached node (rather than invoking a callback at the target),
// and does not yield Progress information.
//
// This function is a helper function which starts a new traversal with default configuration.
// It cannot cross links automatically (since this requires configuration).
// Use the equivalent Get function on the Progress structure
// for more advanced and configurable walks.
func Get(n ipld.Node, p ipld.Path) (ipld.Node, error) {
	return Progress{}.Get(n, p)
}

// FocusedTransform traverses an ipld.Node graph, reaches a single Node,
// and calls the given TransformFn to decide what new node to replace the visited node with.
// A new Node tree will be returned (the original is unchanged).
//
// This function is a helper function which starts a new traversal with default configuration.
// It cannot cross links automatically (since this requires configuration).
// Use the equivalent FocusedTransform function on the Progress structure
// for more advanced and configurable walks.
func FocusedTransform(n ipld.Node, p ipld.Path, fn TransformFn) (ipld.Node, error) {
	return Progress{}.FocusedTransform(n, p, fn)
}

// Focus traverses a Node graph according to a path, reaches a single Node,
// and calls the given VisitFn on that reached node.
//
// Focus is a read-only traversal.
// See FocusedTransform if looking for a way to do an "update" to a Node.
//
// Provide configuration to this process using the Config field in the Progress object.
//
// This walk will automatically cross links, but requires some configuration
// with link loading functions to do so.
//
// Focus (and the other traversal functions) can be used again again inside the VisitFn!
// By using the traversal.Progress handed to the VisitFn,
// the Path recorded of the traversal so far will continue to be extended,
// and thus continued nested uses of Walk and Focus will see the fully contextualized Path.
func (prog Progress) Focus(n ipld.Node, p ipld.Path, fn VisitFn) error {
	n, err := prog.get(n, p, true)
	if err != nil {
		return err
	}
	return fn(prog, n)
}

// Get is the equivalent of Focus, but returns the reached node (rather than invoking a callback at the target),
// and does not yield Progress information.
//
// Provide configuration to this process using the Config field in the Progress object.
//
// This walk will automatically cross links, but requires some configuration
// with link loading functions to do so.
//
// If doing several traversals which are nested, consider using the Focus funcion in preference to Get;
// the Focus functions provide updated Progress objects which can be used to do nested traversals while keeping consistent track of progress,
// such that continued nested uses of Walk or Focus or Get will see the fully contextualized Path.
func (prog Progress) Get(n ipld.Node, p ipld.Path) (ipld.Node, error) {
	return prog.get(n, p, false)
}

// get is the internal implementation for Focus and Get.
// It *mutates* the Progress object it's called on, and returns reached nodes.
// For Get calls, trackProgress=false, which avoids some allocations for state tracking that's not needed by that call.
func (prog *Progress) get(n ipld.Node, p ipld.Path, trackProgress bool) (ipld.Node, error) {
	prog.init()
	segments := p.Segments()
	var prev ipld.Node // for LinkContext
	for i, seg := range segments {
		// Traverse the segment.
		switch n.ReprKind() {
		case ipld.ReprKind_Invalid:
			panic(fmt.Errorf("invalid node encountered at %q", p.Truncate(i)))
		case ipld.ReprKind_Map:
			next, err := n.LookupByString(seg.String())
			if err != nil {
				return nil, fmt.Errorf("error traversing segment %q on node at %q: %s", seg, p.Truncate(i), err)
			}
			prev, n = n, next
		case ipld.ReprKind_List:
			intSeg, err := seg.Index()
			if err != nil {
				return nil, fmt.Errorf("error traversing segment %q on node at %q: the segment cannot be parsed as a number and the node is a list", seg, p.Truncate(i))
			}
			next, err := n.LookupByIndex(intSeg)
			if err != nil {
				return nil, fmt.Errorf("error traversing segment %q on node at %q: %s", seg, p.Truncate(i), err)
			}
			prev, n = n, next
		default:
			return nil, fmt.Errorf("cannot traverse node at %q: %s", p.Truncate(i), fmt.Errorf("cannot traverse terminals"))
		}
		// Dereference any links.
		for n.ReprKind() == ipld.ReprKind_Link {
			lnk, _ := n.AsLink()
			// Assemble the LinkContext in case the Loader or NBChooser want it.
			lnkCtx := ipld.LinkContext{
				LinkPath:   p.Truncate(i),
				LinkNode:   n,
				ParentNode: prev,
			}
			// Pick what in-memory format we will build.
			np, err := prog.Cfg.LinkTargetNodePrototypeChooser(lnk, lnkCtx)
			if err != nil {
				return nil, fmt.Errorf("error traversing node at %q: could not load link %q: %s", p.Truncate(i+1), lnk, err)
			}
			nb := np.NewBuilder()
			// Load link!
			err = lnk.Load(
				prog.Cfg.Ctx,
				lnkCtx,
				nb,
				prog.Cfg.LinkLoader,
			)
			if err != nil {
				return nil, fmt.Errorf("error traversing node at %q: could not load link %q: %s", p.Truncate(i+1), lnk, err)
			}
			if trackProgress {
				prog.LastBlock.Path = p.Truncate(i + 1)
				prog.LastBlock.Link = lnk
			}
			prev, n = n, nb.Build()
		}
	}
	if trackProgress {
		prog.Path = prog.Path.Join(p)
	}
	return n, nil
}

// FocusedTransform traverses an ipld.Node graph, reaches a single Node,
// and calls the given TransformFn to decide what new node to replace the visited node with.
// A new Node tree will be returned (the original is unchanged).
//
// If the TransformFn returns the same Node which it was called with,
// then the transform is a no-op, and the Node returned from the
// FocusedTransform call as a whole will also be the same as its starting Node.
//
// Otherwise, the reached node will be "replaced" with the new Node -- meaning
// that new intermediate nodes will be constructed to also replace each
// parent Node that was traversed to get here, thus propagating the changes in
// a copy-on-write fashion -- and the FocusedTransform function as a whole will
// return a new Node containing identical children except for those replaced.
//
// FocusedTransform can be used again inside the applied function!
// This kind of composition can be useful for doing batches of updates.
// E.g. if have a large Node graph which contains a 100-element list, and
// you want to replace elements 12, 32, and 95 of that list:
// then you should FocusedTransform to the list first, and inside that
// TransformFn's body, you can replace the entire list with a new one
// that is composed of copies of everything but those elements -- including
// using more TransformFn calls as desired to produce the replacement elements
// if it so happens that those replacement elements are easiest to construct
// by regarding them as incremental updates to the previous values.
//
// Note that anything you can do with the Transform function, you can also
// do with regular Node and NodeBuilder usage directly.  Transform just
// does a large amount of the intermediate bookkeeping that's useful when
// creating new values which are partial updates to existing values.
//
// This feature is not yet implemented.
func (prog Progress) FocusedTransform(n ipld.Node, p ipld.Path, fn TransformFn) (ipld.Node, error) {
	panic("TODO") // TODO surprisingly different from Focus -- need to store nodes we traversed, and able do building.
}
