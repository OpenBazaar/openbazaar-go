package ipld

import (
	"context"
	"io"
)

// Link is a special kind of value in IPLD which can be "loaded" to access
// more nodes.
//
// Nodes can return a Link; this can be loaded manually, or,
// the traversal package contains powerful features for automatically
// traversing links through large trees of nodes.
//
// Links straddle somewhat awkwardly across the IPLD Layer Model:
// clearly not at the Schema layer (though schemas can define their parameters),
// partially at the Data Model layer (as they're recognizably in the Node interface),
// and also involved at some serial layer that we don't often talk about:
// linking -- since we're a content-addressed system at heart -- necessarily
// involves understanding of concrete serialization details:
// which encoding mechanisms to use, what string escaping, what hashing, etc,
// and indeed what concrete serial link representation itself to use.
//
// Link is an abstract interface so that we can describe Nodes without
// getting stuck on specific details of any link representation.
// In practice, you'll almost certainly use CIDs for linking.
// However, it's possible to bring your own Link implementations
// (though this'll almost certainly involve also bringing your own encoding
// systems; it's a lot of work).
// It's even possible to use IPLD *entirely without* any linking implementation,
// using it purely for json/cbor via the encoding packages and
// foregoing the advanced traversal features around transparent link loading.
type Link interface {
	// Load consumes serial data from a Loader and funnels the parsed
	// data into a NodeAssembler.
	//
	// The provided Loader function is used to get a reader for the raw
	// serialized content; the Link contains an understanding of how to
	// select a decoder (and hasher for verification, etc); and the
	// NodeAssembler accumulates the final results (which you can
	// presumably access from elsewhere; Load is designed not to know
	// about this).
	Load(context.Context, LinkContext, NodeAssembler, Loader) error

	// LinkBuilder returns a handle to any parameters of the Link which
	// are needed to create a new Link of the same style but with new content.
	// (It's much like the relationship of Node/NodeBuilder.)
	//
	// (If you're familiar with CIDs, you can think of this method as
	// corresponding closely to `cid.Prefix()`, just more abstractly.)
	LinkBuilder() LinkBuilder

	// String should return a reasonably human-readable debug-friendly
	// representation of a Link.  It should only be used for debug and
	// log message purposes; there is no contract that requires that the
	// string be able to be parsed back into a reified Link.
	String() string
}

// LinkBuilder encapsulates any implementation details and parameters
// necessary for taking a Node and converting it to a serial representation
// and returning a Link to that data.
//
// The serialized bytes will be routed through the provided Storer system,
// which is expected to store them in some way such that a related Loader
// system can later use the Link and an associated Loader to load nodes
// of identical content.
//
// LinkBuilder, like Link, is an abstract interface.
// If using CIDs as an implementation, LinkBuilder will encapsulate things
// like multihashType, multicodecType, and cidVersion, for example.
type LinkBuilder interface {
	Build(context.Context, LinkContext, Node, Storer) (Link, error)
}

// Loader functions are used to get a reader for raw serialized content
// based on the lookup information in a Link.
// A loader function is used by providing it to a Link.Load() call.
//
// Loaders typically have some filesystem or database handle contained
// within their closure which is used to satisfy read operations.
//
// LinkContext objects can be provided to give additional information
// to the loader, and will be automatically filled out when a Loader
// is used by systems in the traversal package; most Loader implementations
// should also work fine when given the zero value of LinkContext.
//
// Loaders are implicitly coupled to a Link implementation and have some
// "extra" knowledge of the concrete Link type.  This necessary since there is
// no mandated standard for how to serially represent Link itself, and such
// a representation is typically needed by a Storer implementation.
type Loader func(lnk Link, lnkCtx LinkContext) (io.Reader, error)

// Storer functions are used to a get a writer for raw serialized content,
// which will be committed to storage indexed by Link.
// A stoerer function is used by providing it to a LinkBuilder.Build() call.
//
// The storer system comes in two parts: the Storer itself *starts* a storage
// operation (presumably to some e.g. tempfile) and returns a writer; the
// StoreCommitter returned with the writer is used to *commit* the final storage
// (much like a 'Close' operation for the writer).
//
// Storers typically have some filesystem or database handle contained
// within their closure which is used to satisfy read operations.
//
// LinkContext objects can be provided to give additional information
// to the storer, and will be automatically filled out when a Storer
// is used by systems in the traversal package; most Storer implementations
// should also work fine when given the zero value of LinkContext.
//
// Storers are implicitly coupled to a Link implementation and have some
// "extra" knowledge of the concrete Link type.  This necessary since there is
// no mandated standard for how to serially represent Link itself, and such
// a representation is typically needed by a Storer implementation.
type Storer func(lnkCtx LinkContext) (io.Writer, StoreCommitter, error)

// StoreCommitter is a thunk returned by a Storer which is used to "commit"
// the storage operation.  It should be called after the associated writer
// is finished, similar to a 'Close' method, but further takes a Link parameter,
// which is the identity of the content.  Typically, this will cause an atomic
// operation in the storage system to move the already-written content into
// a final place (e.g. rename a tempfile) determined by the Link.
// (The Link parameter is necessarily only given at the end of the process
// rather than the beginning to so that we can have content-addressible
// semantics while also supporting streaming writes.)
type StoreCommitter func(Link) error

// LinkContext is a parameter to Storer and Loader functions.
//
// An example use of LinkContext might be inspecting the LinkNode, and if
// it's a typed node, inspecting its Type property; then, a Loader might
// deciding on whether or not we want to load objects of that Type.
// This might be used to do a traversal which looks at all directory objects,
// but not file contents, for example.
type LinkContext struct {
	LinkPath   Path
	LinkNode   Node // has the Link again, but also might have type info // always zero for writing new nodes, for obvi reasons.
	ParentNode Node
}

// n.b. if I had java, this would all indeed be generic:
// `Link<$T>`, `LinkBuilder<$T>`, `Storer<$T>`, etc would be an explicit family.
// ... Then again, in java, that'd prevent composition of a Storer or Loader
// which could support more than one concrete type, so.  ¯\_(ツ)_/¯
