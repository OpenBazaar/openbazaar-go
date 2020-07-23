go-ipld-prime
=============

`go-ipld-prime` is an implementation of the IPLD spec interfaces,
a batteries-included codec implementations of IPLD for CBOR and JSON,
and tooling for basic operations on IPLD objects (traversals, etc).



API
---

The API is split into several packages based on responsibly of the code.
The most central interfaces are the base package,
but you'll certainly need to import additional packages to get concrete implementations into action.

Roughly speaking, the core package interfaces are all about the IPLD Data Model;
the codec packages contain functions for parsing serial data into the IPLD Data Model,
and converting Data Model content back into serial formats;
the traversal package is an example of higher-order functions on the Data Model;
concrete 'Node' implementations ready to use can be found under 'impl/*';
and several additional packages contain advanced features such as IPLD Schemas.

(Because the codecs, as well as higher-order features like traversals, are
implemented in a separate package from the core interfaces or any of the Node implementations,
you can be sure they're not doing any funky "magic" -- all this stuff will work the same
if you want to write your own extensions, whether for new Node implementations
or new codecs, or new higher-order order functions!)

- `github.com/ipld/go-ipld-prime` -- imported as just `ipld` -- contains the core interfaces for IPLD.  The most important interfaces are `Node`, `NodeBuilder`, `Path`, and `Link`.
- `github.com/ipld/go-ipld-prime/impl/free` -- imported as `ipldfree` -- provides concrete implementations of `Node` and `NodeBuilder` which work for any kind of data.
- `github.com/ipld/go-ipld-prime/impl/cbor` -- imported as `ipldcbor` -- provides concrete implementations of `Node` and `NodeBuilder` which have some special features to accelerate certain workloads with CBOR.
- `github.com/ipld/go-ipld-prime/traversal` -- contains higher-order functions for traversing graphs of data easily.
- `github.com/ipld/go-ipld-prime/traversal/selector` -- contains selectors, which are sort of like regexps, but for trees and graphs of IPLD data!
- `github.com/ipld/go-ipld-prime/codec -- parent package of all the codec implementations!
- `github.com/ipld/go-ipld-prime/codec/dagcbor` -- implementations of marshalling and unmarshalling as CBOR (a fast, binary serialization format).
- `github.com/ipld/go-ipld-prime/codec/dagjson` -- implementations of marshalling and unmarshalling as JSON (a popular human readable format).
- `github.com/ipld/go-ipld-prime/linking/cid` -- imported as `cidlink` -- provides concrete implementations of `Link` as a CID.  Also, the multicodec registry.
- `github.com/ipld/go-ipld-prime/schema` -- contains the `schema.Type` and `schema.TypedNode` interface declarations, which represent IPLD Schema type information.
- `github.com/ipld/go-ipld-prime/impl/typed` -- provides concrete implementations of `schema.TypedNode` which decorate a basic `Node` at runtime to have additional features described by IPLD Schemas.



Other IPLD Libraries
--------------------

The IPLD specifications are designed to be language-agnostic.
Many implementations exist in a variety of languages.

For overall behaviors and specifications, refer to the specs repo:
  https://github.com/ipld/specs/


### distinctions from go-ipld-interface&go-ipld-cbor

This library ("go ipld prime") is the current head of development for golang IPLD,
but several other libraries exist which are widely deployed.

This library is a clean take on the IPLD interfaces and addresses several design decisions very differently than existing libraries:

- The Node interfaces are minimal (and match cleanly to the IPLD Data Model);
- Many features known to be legacy are dropped;
- The Link implementations are purely CIDs;
- The Path implementations are provided in the same box;
- The JSON and CBOR implementations are provided in the same box;
- And several odd dependencies on blockstore and other interfaces from the rest of the IPFS ecosystem are removed.

Many of these changes had been discussed for the other IPLD codebases as well,
but we chose clean break v2 as a more viable project-management path.
Both the existing IPLD libraries and go-ipld-prime can co-exist on the same import path, and refer to the same kinds of serial data.
Projects wishing to migrate can do so smoothly and at their leisure.

There is no explicit deprecation timeline for the earlier golang IPLD libraries,
but you should expect new features *here*, rather than in those libraries.



Change Policy
-------------

The go-ipld-prime library is already usable.  We are also still in development, and may still change things.

Using a commit hash when depending on this library is advisable (as it is with any other).

We may sometimes tag releases, but it's just as acceptable to track commits on master without the indirection.

The following are all norms you can expect of changes to this codebase:

- The `master` branch will not be force-pushed.
    - (exceptional circumstances may exist, but such exceptions will only be considered valid for about as long after push as the "$N-second-rule" about dropped food).
    - Therefore, commit hashes on master are gold to link against.
- All other branches *will* be force-pushed.
    - Therefore, commit hashes not reachable from the master branch are inadvisable to link against.
- If it's on master, it's understood to be good, in as much as we can tell.
- Development proceeds -- both starting from and ending on -- the `master` branch.
    - There are no other long-running supported-but-not-master branches.
    - The existence of tags at any particular commit do not indicate that we will consider starting a long running and supported diverged branch from that point, nor start doing backports, etc.
- All changes are presumed breaking until proven otherwise; and we don't have the time and attention budget at this point for doing the "proven otherwise".
    - All consumers updating their libraries should run their own compiler, linking, and test suites before assuming the update applies cleanly -- as is good practice regardless.
    - Any idea of semver indicating more or less breakage should be treated as a street vendor selling potions of levitation -- it's likely best disregarded.

None of this is to say we'll go breaking things willy-nilly for fun; but it *is* to say:

- Staying close to master is always better than not staying close to master;
- and trust your compiler and your tests rather than tea-leaf patterns in a tag string.
